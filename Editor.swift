import AppKit
import Sparkle
import SwiftUI

private struct SaveChoice: Decodable, Identifiable {
    let path: String
    let label: String
    var id: String { path }
}

private struct GarageChoice: Decodable, Identifiable {
    let id: String
    let city: String
    let status: String
    let canBuy: Bool

    var label: String {
        switch status {
        case "0": return canBuy ? "Satın alınmamış" : "Kullanılamıyor"
        case "2": return "Küçük · 3 yer"
        case "3": return "Büyük · 5 yer"
        case "6": return "Başlangıç · 1 yer"
        default: return "Durum: \(status)"
        }
    }
}

private struct SaveState: Decodable {
    let garages: [GarageChoice]
    let unvisitedCities: Int
    let saves: [SaveChoice]
    let path: String
    let token: String
    let money: String
    let skills: [Int]
    let xp: Int
    let level: Int
    let canRepair: Bool
    let canRefuel: Bool
    let job: String
    let canCancel: Bool
    let gameOpen: Bool
    let backup: String?
}

private struct EditorError: Decodable, LocalizedError {
    let error: String
    var errorDescription: String? { error }
}

private func runEditor<Result: Decodable>(_ arguments: [String], as type: Result.Type) throws -> Result {
    let task = Process()
    task.executableURL = Bundle.main.bundleURL.appendingPathComponent("Contents/Helpers/ets2-save-core")
    task.arguments = arguments
    let output = Pipe()
    task.standardOutput = output
    task.standardError = FileHandle.nullDevice
    try task.run()
    let data = output.fileHandleForReading.readDataToEndOfFile()
    task.waitUntilExit()
    let decoder = JSONDecoder()
    if let failure = try? decoder.decode(EditorError.self, from: data) { throw failure }
    guard task.terminationStatus == 0,
          let result = try? decoder.decode(type, from: data) else {
        throw EditorError(error: "Kayıt okunamadı.")
    }
    return result
}

private struct Editor: View {
    private let names = ["ADR", "Uzun Mesafe", "Değerli Yük", "Kırılgan Yük", "Zamanında Teslimat", "Ekonomik Sürüş"]
    private let classes = ["Patlayıcılar", "Gazlar", "Yanıcı sıvılar", "Yanıcı katılar", "Zehirli maddeler", "Aşındırıcı maddeler"]
    @State private var save: SaveState?
    @State private var money = ""
    @State private var skills = Array(repeating: 0, count: 6)
    @State private var cancelJob = false
    @State private var repair = false
    @State private var refuel = false
    @State private var careerMode = "level"
    @State private var career = ""
    @State private var showSteam = false
    @State private var showGarages = false
    @State private var garagePurchases: Set<String> = []
    @State private var visitCities = false
    @State private var status = ""
    @State private var failed = false
    @State private var backup: String?
    @State private var busy = false

    var body: some View {
        VStack(alignment: .leading, spacing: 13) {
            HStack(spacing: 8) {
                Text(saveLabel).font(.callout).foregroundStyle(.secondary)
                    .lineLimit(1).truncationMode(.middle).help(save?.path ?? "")
                Spacer()
                Button("Son Kayıt") { load(["read"]) }
                    .help("En yeni ETS2 kaydını yeniden yükle")
                Button("Seç…", action: chooseSave)
            }
            .disabled(busy)

            if let save, !save.saves.isEmpty {
                Menu("Bulunan Kayıtlar") {
                    ForEach(save.saves) { choice in
                        Button(choice.label) { load(["read", choice.path]) }
                            .disabled(choice.path == save.path)
                    }
                }
                .disabled(busy)
            }

            Grid(alignment: .leading, horizontalSpacing: 22, verticalSpacing: 9) {
                GridRow {
                    Text("Bakiye")
                    HStack(spacing: 5) {
                        TextField("Bakiye", text: $money)
                            .labelsHidden()
                            .textFieldStyle(.roundedBorder)
                            .frame(width: 105)
                        Text("€").foregroundStyle(.secondary)
                    }
                }
                GridRow {
                    Picker("Kariyer", selection: $careerMode) {
                        Text("Seviye").tag("level")
                        Text("XP").tag("xp")
                    }
                    .labelsHidden()
                    .fixedSize()
                    .onChange(of: careerMode) { mode in
                        if let save { career = String(mode == "level" ? save.level : save.xp) }
                    }
                    TextField(careerMode == "level" ? "Seviye" : "XP", text: $career)
                        .textFieldStyle(.roundedBorder)
                        .frame(width: 105)
                }
                GridRow {
                    Text("ADR")
                    Menu("\(skills[0].nonzeroBitCount) / 6 sınıf") {
                        ForEach(classes.indices, id: \.self) { index in
                            Toggle(classes[index], isOn: adrClass(index))
                        }
                    }
                    .frame(width: 128, alignment: .leading)
                    Button("Maks") { skills[0] = 63 }
                        .accessibilityLabel("Tüm ADR sınıflarını aç")
                        .disabled(skills[0] == 63)
                }
                ForEach(1..<names.count, id: \.self) { index in
                    GridRow {
                        Text(names[index])
                        HStack(spacing: 8) {
                            Text("\(skills[index]) / 6").monospacedDigit()
                                .frame(minWidth: 35, alignment: .leading)
                            Stepper(names[index], value: $skills[index], in: 0...6)
                                .labelsHidden()
                                .accessibilityLabel(names[index])
                        }
                        Button("Maks") { skills[index] = 6 }
                            .accessibilityLabel("\(names[index]) becerisini en yüksek seviyeye çıkar")
                            .disabled(skills[index] == 6)
                    }
                }
            }
            .disabled(save == nil || busy)

            if let save, save.unvisitedCities > 0 {
                Toggle("Hızlı yolculuk için şehirleri aç (\(save.unvisitedCities))", isOn: $visitCities)
                    .toggleStyle(.checkbox)
                    .help("Kayıttaki şehirleri ziyaret edilmiş olarak işaretler.")
                    .disabled(busy)
            }

            if let save, save.canRepair || save.canRefuel {
                HStack(spacing: 15) {
                    if save.canRepair {
                        Toggle("Tır ve dorseyi onar", isOn: $repair)
                            .help("Hasar ve kalıcı aşınmayı sıfırlar.")
                    }
                    if save.canRefuel {
                        Toggle("Depoyu doldur", isOn: $refuel)
                    }
                }
                .toggleStyle(.checkbox)
                .disabled(busy || cancelJob)
            }

            if let save, save.canCancel {
                Toggle("Hızlı işi cezasız iptal et", isOn: $cancelJob)
                    .toggleStyle(.checkbox)
                    .help(save.job)
                    .disabled(busy || repair || refuel)
            } else if let save, !save.job.isEmpty {
                Text("Bu iş için uygun iş öncesi kayıt bulunamadı.")
                    .font(.caption).foregroundStyle(.secondary)
            }

            if !status.isEmpty {
                Text(status).font(.caption)
                    .foregroundStyle(failed ? Color(nsColor: .systemRed) : .secondary)
                    .fixedSize(horizontal: false, vertical: true)
            }

            HStack(spacing: 12) {
                Button("Steam Ayarları…") { showSteam = true }
                if let save, !save.garages.isEmpty {
                    Button(garagePurchases.isEmpty ? "Garajlar…" : "Garajlar (\(garagePurchases.count))") {
                        showGarages = true
                    }
                    .disabled(busy)
                }
            }

            HStack {
                if busy {
                    ProgressView().controlSize(.small)
                        .accessibilityLabel("Kayıt işleniyor")
                }
                if let backup {
                    Button("Yedeği Göster") {
                        NSWorkspace.shared.activateFileViewerSelecting([URL(fileURLWithPath: backup)])
                    }
                }
                Spacer()
                Button("Kaydet", action: saveChanges)
                    .keyboardShortcut("s", modifiers: .command)
                    .buttonStyle(.borderedProminent)
                    .disabled(!canSave || busy)
            }
        }
        .padding(19)
        .frame(width: 406)
        .sheet(isPresented: $showSteam) { SteamOptions() }
        .sheet(isPresented: $showGarages) {
            if let save {
                GarageOptions(garages: save.garages, selected: $garagePurchases)
            }
        }
        .task { if save == nil && status.isEmpty { load(["read"]) } }
    }

    private var saveLabel: String {
        guard let save else { return "Kayıt seçilmedi" }
        let slot = URL(fileURLWithPath: save.path)
        let encoded = slot.deletingLastPathComponent().deletingLastPathComponent().lastPathComponent
        var bytes = [UInt8]()
        var index = encoded.startIndex
        while let next = encoded.index(index, offsetBy: 2, limitedBy: encoded.endIndex), index < next,
              let byte = UInt8(encoded[index..<next], radix: 16) {
            bytes.append(byte)
            index = next
        }
        let profile = index == encoded.endIndex ? String(bytes: bytes, encoding: .utf8) : nil
        let name = slot.lastPathComponent == "autosave" ? "Otomatik kayıt" : slot.lastPathComponent
        return profile.map { "\($0) · \(name)" } ?? name
    }

    private var careerTarget: Int? {
        guard let n = Int(career), n >= 0, n <= (careerMode == "level" ? 315821 : 2147483647) else { return nil }
        return n
    }

    private var canSave: Bool {
        guard let save, let amount = Int64(money), let target = careerTarget, !save.gameOpen else { return false }
        let original = careerMode == "level" ? save.level : save.xp
        return String(amount) != save.money || skills != save.skills || target != original || cancelJob || repair || refuel || !garagePurchases.isEmpty || visitCities
    }

    private func adrClass(_ index: Int) -> Binding<Bool> {
        let mask = 1 << index
        return Binding(
            get: { skills[0] & mask != 0 },
            set: { enabled in
                if enabled { skills[0] |= mask }
                else { skills[0] &= ~mask }
            }
        )
    }

    private func chooseSave() {
        let panel = NSOpenPanel()
        panel.title = "ETS2 kayıt klasörünü veya game.sii dosyasını seç"
        panel.canChooseDirectories = true
        panel.canChooseFiles = true
        panel.allowsMultipleSelection = false
        if let save {
            panel.directoryURL = URL(fileURLWithPath: save.path)
        } else {
            panel.directoryURL = FileManager.default.homeDirectoryForCurrentUser
                .appendingPathComponent("Library/Application Support")
        }
        if panel.runModal() == .OK, let url = panel.url { load(["read", url.path]) }
    }

    private func saveChanges() {
        guard let save, canSave, let amount = Int64(money), let target = careerTarget else { return }
        load(["write", save.path, save.token, String(amount)] + skills.map { String($0) }
             + [cancelJob ? "cancel" : "keep", "\(careerMode):\(target)", String(repair), String(refuel)]
             + (visitCities ? ["visit-cities"] : []) + garagePurchases.sorted())
    }

    private func load(_ arguments: [String]) {
        guard !busy else { return }
        busy = true
        status = ""
        failed = false
        Task { @MainActor in
            defer { busy = false }
            do {
                let result = try await Task.detached(priority: .userInitiated) {
                    try runEditor(arguments, as: SaveState.self)
                }.value
                guard result.skills.count == 6 else {
                    throw EditorError(error: "Beceriler okunamadı.")
                }
                save = result
                money = result.money
                skills = result.skills
                cancelJob = false
                garagePurchases.removeAll()
                visitCities = false
                repair = false
                refuel = false
                career = String(careerMode == "level" ? result.level : result.xp)
                backup = result.backup
                if result.gameOpen {
                    status = "Kaydetmek için ETS2’yi kapat ve Son Kayıt’a bas."
                } else if result.backup != nil {
                    status = "Kaydedildi. ETS2’de bu kaydı yükle."
                }
            } catch {
                status = error.localizedDescription
                failed = true
            }
        }
    }
}

private struct GarageOptions: View {
    let garages: [GarageChoice]
    @Binding var selected: Set<String>
    @Environment(\.dismiss) private var dismiss
    @State private var search = ""

    private var buyable: Set<String> { Set(garages.filter(\.canBuy).map(\.id)) }
    private var visible: [GarageChoice] {
        search.isEmpty ? garages : garages.filter { $0.city.localizedStandardContains(search) }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Garajlar").font(.headline)
            Text("Seçilen garajlar Kaydet’e basınca 5 yerli olarak açılır. Bakiyeden ücret düşmez.")
                .font(.callout).foregroundStyle(.secondary)
            TextField("Şehir ara", text: $search).textFieldStyle(.roundedBorder)
            HStack {
                Button("Alınabileceklerin tümünü seç") { selected = buyable }
                    .disabled(buyable.isEmpty || selected == buyable)
                Button("Seçimi temizle") { selected.removeAll() }
                    .disabled(selected.isEmpty)
                Spacer()
                Text("\(selected.count) seçili").foregroundStyle(.secondary)
            }
            List(visible) { garage in
                HStack {
                    if garage.canBuy {
                        Toggle(garage.city.capitalized, isOn: selection(for: garage))
                            .toggleStyle(.checkbox)
                    } else {
                        Text(garage.city.capitalized)
                    }
                    Spacer()
                    Text(garage.label).foregroundStyle(.secondary)
                }
            }
            .frame(height: 270)
            HStack {
                Spacer()
                Button("Bitti") { dismiss() }.keyboardShortcut(.defaultAction)
            }
        }
        .padding(20)
        .frame(width: 460)
    }

    private func selection(for garage: GarageChoice) -> Binding<Bool> {
        Binding(get: { selected.contains(garage.id) }, set: { enabled in
            if enabled { selected.insert(garage.id) }
            else { selected.remove(garage.id) }
        })
    }
}

private struct SteamState: Decodable {
    let enabled: Bool
    let running: Bool
    let token: String
}

private struct SteamOptions: View {
    @Environment(\.dismiss) private var dismiss
    @State private var steam: SteamState?
    @State private var busy = false
    @State private var error = ""

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Steam Başlatma Seçenekleri").font(.headline)
            Toggle("Vulkan + SDR", isOn: Binding(
                get: { steam?.enabled ?? false },
                set: { enabled in
                    guard let steam else { return }
                    update([enabled ? "on" : "off", steam.token])
                }
            ))
            .toggleStyle(.switch)
            .disabled(steam == nil || steam?.running == true || busy)
            Text("-color_mode sdr -rdevice vk")
                .font(.system(.caption, design: .monospaced))
                .foregroundStyle(.secondary)

            if steam?.enabled == true {
                Text("Vulkan açıkken %100 ölçekleme siyah ekrana neden olabilir. Oyunda %100 dışında bir ölçek seç.")
                    .font(.callout)
                    .fixedSize(horizontal: false, vertical: true)
            }
            if steam?.running == true {
                Text("Ayarı değiştirmek için Steam’i tamamen kapat ve Yenile’ye bas.")
                    .font(.callout).foregroundStyle(.secondary)
            }
            if !error.isEmpty {
                Text(error).font(.callout).foregroundStyle(.red)
            }
            HStack {
                Button("Yenile") { update([]) }.disabled(busy)
                if busy { ProgressView().controlSize(.small) }
                Spacer()
                Button("Kapat") { dismiss() }.keyboardShortcut(.cancelAction)
            }
        }
        .padding(20)
        .frame(width: 390)
        .task { update([]) }
    }

    private func update(_ arguments: [String]) {
        guard !busy else { return }
        busy = true
        error = ""
        Task { @MainActor in
            defer { busy = false }
            do {
                steam = try await Task.detached {
                    try runEditor(["steam"] + arguments, as: SteamState.self)
                }.value
            } catch {
                self.error = error.localizedDescription
            }
        }
    }
}

@main
struct SaveEditorApp: App {
    private let updater = SPUStandardUpdaterController(
        startingUpdater: true, updaterDelegate: nil, userDriverDelegate: nil
    )

    var body: some Scene {
        Window("ETS2 Save Editor", id: "editor") {
            Editor()
        }
        .windowResizability(.contentSize)
        .commands {
            CommandGroup(after: .appInfo) {
                Button("Güncellemeleri Denetle…") { updater.checkForUpdates(nil) }
            }
        }
    }
}
