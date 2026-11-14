import AppKit
import SwiftUI

private struct SaveChoice: Decodable, Identifiable {
    let path: String
    let label: String
    var id: String { path }
}

private struct SaveState: Decodable {
    let saves: [SaveChoice]
    let path: String
    let token: String
    let money: String
    let skills: [Int]
    let job: String
    let canCancel: Bool
    let gameOpen: Bool
    let backup: String?
}

private struct EditorError: Decodable, LocalizedError {
    let error: String
    var errorDescription: String? { error }
}

private func runEditor(_ arguments: [String]) throws -> SaveState {
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
          let save = try? decoder.decode(SaveState.self, from: data),
          save.skills.count == 6 else {
        throw EditorError(error: "Kayıt okunamadı.")
    }
    return save
}

private struct Editor: View {
    private let names = ["ADR", "Uzun Mesafe", "Değerli Yük", "Kırılgan Yük", "Zamanında Teslimat", "Ekonomik Sürüş"]
    private let classes = ["Patlayıcılar", "Gazlar", "Yanıcı sıvılar", "Yanıcı katılar", "Zehirli maddeler", "Aşındırıcı maddeler"]
    @State private var save: SaveState?
    @State private var money = ""
    @State private var skills = Array(repeating: 0, count: 6)
    @State private var cancelJob = false
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
                    Text("ADR")
                    Menu("\(skills[0].nonzeroBitCount) / 6 sınıf") {
                        ForEach(classes.indices, id: \.self) { index in
                            Toggle(classes[index], isOn: adrClass(index))
                        }
                    }
                    .frame(width: 128, alignment: .leading)
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
                    }
                }
            }
            .disabled(save == nil || busy)

            if let save, save.canCancel {
                Toggle("Hızlı işi cezasız iptal et", isOn: $cancelJob)
                    .toggleStyle(.checkbox)
                    .help(save.job)
                    .disabled(busy)
            } else if let save, !save.job.isEmpty {
                Text("Bu iş için uygun iş öncesi kayıt bulunamadı.")
                    .font(.caption).foregroundStyle(.secondary)
            }

            if !status.isEmpty {
                Text(status).font(.caption)
                    .foregroundStyle(failed ? Color(nsColor: .systemRed) : .secondary)
                    .fixedSize(horizontal: false, vertical: true)
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

    private var canSave: Bool {
        guard let save, let amount = Int64(money), !save.gameOpen else { return false }
        return String(amount) != save.money || skills != save.skills || cancelJob
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
        guard let save, canSave, let amount = Int64(money) else { return }
        load(["write", save.path, save.token, String(amount)] + skills.map { String($0) }
             + [cancelJob ? "cancel" : "keep"])
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
                    try runEditor(arguments)
                }.value
                save = result
                money = result.money
                skills = result.skills
                cancelJob = false
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

@main
struct MiniEditorApp: App {
    var body: some Scene {
        Window("ETS2 Mini", id: "editor") {
            Editor()
        }
        .windowResizability(.contentSize)
    }
}
