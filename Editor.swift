import AppKit
import Sparkle
import SwiftUI

private let prefersTurkish = Locale.preferredLanguages.first?
    .split(separator: "-").first?.lowercased() == "tr"

private let translationBundle = prefersTurkish
    ? Bundle.main.url(forResource: "tr", withExtension: "lproj").flatMap { Bundle(url: $0) }
    : nil

private func tr(_ key: String, _ arguments: CVarArg...) -> String {
    let translation = translationBundle?.localizedString(forKey: key, value: key, table: nil) ?? key
    return arguments.isEmpty ? translation : String(format: translation, arguments: arguments)
}

@MainActor
private func showAbout() {
    let credits = NSMutableAttributedString(string: tr("A save editor for Euro Truck Simulator 2.") + "\n\n")
    let repository = URL(string: "https://github.com/code-a-man/ets2-save-editor")!
    credits.append(NSAttributedString(string: "GitHub", attributes: [.link: repository]))
    credits.append(NSAttributedString(string: " · MIT License"))
    NSApp.orderFrontStandardAboutPanel(options: [.credits: credits])
}

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
        case "0": return canBuy ? tr("Not owned") : tr("Unavailable")
        case "2": return tr("Small · 3 slots")
        case "3": return tr("Large · 5 slots")
        case "6": return tr("Starter · 1 slot")
        default: return tr("Status: %@", status)
        }
    }
}

private struct SaveState: Decodable {
    let garages: [GarageChoice]
    let undiscoveredDealers: Int
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

private struct AcademyResult: Decodable {
    let backup: String?
    let added: Int
}

private struct EditorError: Decodable, LocalizedError {
    let error: String
    var errorDescription: String? { error }
}

private func runEditor<Result: Decodable>(_ arguments: [String], as type: Result.Type) throws -> Result {
    let task = Process()
    task.executableURL = Bundle.main.bundleURL.appendingPathComponent("Contents/Helpers/ets2-save-core")
    task.arguments = arguments
    var environment = ProcessInfo.processInfo.environment
    environment["ETS2_LANGUAGE"] = prefersTurkish ? "tr" : "en"
    task.environment = environment
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
        throw EditorError(error: tr("Could not read the save."))
    }
    return result
}

private struct Editor: View {
    let checkForUpdates: () -> Void
    private let names = ["ADR", tr("Long Distance"), tr("High Value Cargo"), tr("Fragile Cargo"), tr("Just-in-Time Delivery"), tr("Eco Driving")]
    private let classes = [tr("Explosives"), tr("Gases"), tr("Flammable liquids"), tr("Flammable solids"), tr("Toxic substances"), tr("Corrosive substances")]
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
    @State private var discoverDealers = false
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
                Button(tr("Latest Save")) { load(["read"]) }
                    .help(tr("Reload the latest ETS2 save"))
                Button(tr("Choose…"), action: chooseSave)
            }
            .disabled(busy)

            if let save, !save.saves.isEmpty {
                Menu(tr("Found Saves")) {
                    ForEach(save.saves) { choice in
                        Button(choice.label) { load(["read", choice.path]) }
                            .disabled(choice.path == save.path)
                    }
                }
                .disabled(busy)
            }

            Grid(alignment: .leading, horizontalSpacing: 22, verticalSpacing: 9) {
                GridRow {
                    Text(tr("Balance"))
                    HStack(spacing: 5) {
                        TextField(tr("Balance"), text: $money)
                            .labelsHidden()
                            .textFieldStyle(.roundedBorder)
                            .frame(width: 105)
                        Text("€").foregroundStyle(.secondary)
                    }
                }
                GridRow {
                    Picker(tr("Career"), selection: $careerMode) {
                        Text(tr("Level")).tag("level")
                        Text("XP").tag("xp")
                    }
                    .labelsHidden()
                    .fixedSize()
                    .onChange(of: careerMode) { mode in
                        if let save { career = String(mode == "level" ? save.level : save.xp) }
                    }
                    TextField(careerMode == "level" ? tr("Level") : "XP", text: $career)
                        .textFieldStyle(.roundedBorder)
                        .frame(width: 105)
                }
                GridRow {
                    Text("ADR")
                    Menu(tr("%lld / 6 classes", skills[0].nonzeroBitCount)) {
                        ForEach(classes.indices, id: \.self) { index in
                            Toggle(classes[index], isOn: adrClass(index))
                        }
                    }
                    .frame(width: 128, alignment: .leading)
                    Button(tr("Max")) { skills[0] = 63 }
                        .accessibilityLabel(tr("Unlock all ADR classes"))
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
                        Button(tr("Max")) { skills[index] = 6 }
                            .accessibilityLabel(tr("Max out %@", names[index]))
                            .disabled(skills[index] == 6)
                    }
                }
            }
            .disabled(save == nil || busy)

            if let save, save.unvisitedCities > 0 {
                Toggle(tr("Unlock cities for quick travel (%lld)", save.unvisitedCities), isOn: $visitCities)
                    .toggleStyle(.checkbox)
                    .help(tr("Marks cities in this save as visited."))
                    .disabled(busy)
            }

            if let save, save.undiscoveredDealers > 0 {
                Toggle(tr("Discover truck dealerships (%lld)", save.undiscoveredDealers), isOn: $discoverDealers)
                    .toggleStyle(.checkbox)
                    .disabled(busy)
            }

            if let save, save.canRepair || save.canRefuel {
                HStack(spacing: 15) {
                    if save.canRepair {
                        Toggle(tr("Repair truck and trailer"), isOn: $repair)
                            .help(tr("Clears damage and permanent wear."))
                    }
                    if save.canRefuel {
                        Toggle(tr("Refuel truck"), isOn: $refuel)
                    }
                }
                .toggleStyle(.checkbox)
                .disabled(busy || cancelJob)
            }

            if let save, save.canCancel {
                Toggle(tr("Cancel Quick Job without a fee"), isOn: $cancelJob)
                    .toggleStyle(.checkbox)
                    .help(save.job)
                    .disabled(busy || repair || refuel)
            } else if let save, !save.job.isEmpty {
                Text(tr("No suitable save from before this job was found."))
                    .font(.caption).foregroundStyle(.secondary)
            }

            if save != nil {
                Button(tr("Complete Driving Academy"), action: completeAcademy)
                    .help(tr("Completes the three Academy modules for this profile."))
                    .disabled(busy)
            }

            if !status.isEmpty {
                Text(status).font(.caption)
                    .foregroundStyle(failed ? Color(nsColor: .systemRed) : .secondary)
                    .fixedSize(horizontal: false, vertical: true)
            }

            HStack(spacing: 12) {
                Button(tr("Steam Settings…")) { showSteam = true }
                if let save, !save.garages.isEmpty {
                    Button(garagePurchases.isEmpty ? tr("Garages…") : tr("Garages (%lld)", garagePurchases.count)) {
                        showGarages = true
                    }
                    .disabled(busy)
                }
                Spacer()
                Button(tr("Check for Updates…"), action: checkForUpdates)
            }

            HStack {
                Button(tr("About")) { showAbout() }
                if busy {
                    ProgressView().controlSize(.small)
                        .accessibilityLabel(tr("Processing save"))
                }
                if let backup {
                    Button(tr("Show Backup")) {
                        NSWorkspace.shared.activateFileViewerSelecting([URL(fileURLWithPath: backup)])
                    }
                }
                Spacer()
                if backup != nil && !hasPendingChanges {
                    Button(tr("Launch Game"), action: launchGame)
                        .buttonStyle(.borderedProminent)
                        .disabled(busy)
                } else {
                    Button(tr("Save"), action: saveChanges)
                        .keyboardShortcut("s", modifiers: .command)
                        .buttonStyle(.borderedProminent)
                        .disabled(!canSave || busy)
                }
            }
        }
        .padding(19)
        .frame(width: 448)
        .sheet(isPresented: $showSteam) { SteamOptions() }
        .sheet(isPresented: $showGarages) {
            if let save {
                GarageOptions(garages: save.garages, selected: $garagePurchases)
            }
        }
        .task { if save == nil && status.isEmpty { load(["read"]) } }
    }

    private var saveLabel: String {
        guard let save else { return tr("No save selected") }
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
        let name = slot.lastPathComponent == "autosave" ? tr("Autosave") : slot.lastPathComponent
        return profile.map { "\($0) · \(name)" } ?? name
    }

    private var careerTarget: Int? {
        guard let n = Int(career), n >= 0, n <= (careerMode == "level" ? 315821 : 2147483647) else { return nil }
        return n
    }

    private var hasPendingChanges: Bool {
        guard let save else { return false }
        let original = careerMode == "level" ? save.level : save.xp
        return Int64(money).map { String($0) } != save.money || skills != save.skills ||
            careerTarget != original || cancelJob || repair || refuel ||
            !garagePurchases.isEmpty || visitCities || discoverDealers
    }

    private var canSave: Bool {
        guard let save, !save.gameOpen, Int64(money) != nil, careerTarget != nil else { return false }
        return hasPendingChanges
    }

    private func launchGame() {
        guard let url = URL(string: "steam://rungameid/227300"), NSWorkspace.shared.open(url) else {
            status = tr("Could not open ETS2. Start it from Steam.")
            failed = true
            return
        }
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
        panel.title = tr("Choose an ETS2 save folder or game.sii")
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

    private func completeAcademy() {
        guard let save, !busy else { return }
        busy = true
        failed = false
        status = ""
        Task { @MainActor in
            defer { busy = false }
            do {
                let result = try await Task.detached(priority: .userInitiated) {
                    try runEditor(["academy", save.path], as: AcademyResult.self)
                }.value
                if let academyBackup = result.backup {
                    backup = academyBackup
                }
                status = result.added > 0
                    ? tr("Driving Academy updated.")
                    : tr("Driving Academy is already complete.")
            } catch {
                status = error.localizedDescription
                failed = true
            }
        }
    }

    private func saveChanges() {
        guard let save, canSave, let amount = Int64(money), let target = careerTarget else { return }
        var arguments = ["write", save.path, save.token, String(amount)]
        arguments.append(contentsOf: skills.map(String.init))
        arguments.append(contentsOf: [
            cancelJob ? "cancel" : "keep", "\(careerMode):\(target)", String(repair), String(refuel)
        ])
        if visitCities { arguments.append("visit-cities") }
        if discoverDealers { arguments.append("discover-dealers") }
        arguments.append(contentsOf: garagePurchases.sorted())
        load(arguments)
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
                    throw EditorError(error: tr("Could not read the skills."))
                }
                save = result
                money = result.money
                skills = result.skills
                cancelJob = false
                garagePurchases.removeAll()
                visitCities = false
                discoverDealers = false
                repair = false
                refuel = false
                career = String(careerMode == "level" ? result.level : result.xp)
                backup = result.backup
                if result.gameOpen {
                    status = tr("Close ETS2 and click Latest Save before saving.")
                } else if result.backup != nil {
                    status = tr("Saved. Load this save in ETS2.")
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
            Text(tr("Garages")).font(.headline)
            Text(tr("Save to open the selected garages with five slots each at no cost."))
                .font(.callout).foregroundStyle(.secondary)
            TextField(tr("Search cities"), text: $search).textFieldStyle(.roundedBorder)
            HStack {
                Button(tr("Select All Available")) { selected = buyable }
                    .disabled(buyable.isEmpty || selected == buyable)
                Button(tr("Clear Selection")) { selected.removeAll() }
                    .disabled(selected.isEmpty)
                Spacer()
                Text(tr("%lld selected", selected.count)).foregroundStyle(.secondary)
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
                Button(tr("Done")) { dismiss() }.keyboardShortcut(.defaultAction)
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
            Text(tr("Steam Launch Options")).font(.headline)
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
                Text(tr("Vulkan can show a black screen at 100% scaling. Choose a different scaling value in ETS2."))
                    .font(.callout)
                    .fixedSize(horizontal: false, vertical: true)
            }
            if steam?.running == true {
                Text(tr("Quit Steam completely and click Refresh to change this setting."))
                    .font(.callout).foregroundStyle(.secondary)
            }
            if !error.isEmpty {
                Text(error).font(.callout).foregroundStyle(.red)
            }
            HStack {
                Button(tr("Refresh")) { update([]) }.disabled(busy)
                if busy { ProgressView().controlSize(.small) }
                Spacer()
                Button(tr("Close")) { dismiss() }.keyboardShortcut(.cancelAction)
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
            Editor { updater.checkForUpdates(nil) }
        }
        .windowResizability(.contentSize)
        .commands {
            CommandGroup(replacing: .appInfo) {
                Button(tr("About ETS2 Save Editor")) { showAbout() }
                Button(tr("Check for Updates…")) { updater.checkForUpdates(nil) }
            }
        }
    }
}
