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

    var isOwned: Bool { ["2", "3", "6"].contains(status) }

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
    let citiesComplete: Bool
    let dealersComplete: Bool
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
    let academyComplete: Bool
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

private let graphite = Color(red: 0.126, green: 0.129, blue: 0.14)
private let titleSurface = Color(red: 0.173, green: 0.176, blue: 0.189)
private let inputSurface = Color(red: 0.235, green: 0.239, blue: 0.252)
private let amber = Color(red: 1, green: 0.69, blue: 0.12)

private extension View {
    func controlSurface() -> some View {
        background(inputSurface, in: RoundedRectangle(cornerRadius: 5))
            .overlay {
                RoundedRectangle(cornerRadius: 5).strokeBorder(.white.opacity(0.16))
                    .allowsHitTesting(false)
            }
    }
}

private struct EditorButton: ButtonStyle {
    var prominent = false
    @Environment(\.isEnabled) private var enabled

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(.system(size: prominent ? 14 : 12, weight: prominent ? .bold : .medium))
            .foregroundStyle(prominent ? Color.black : Color.primary)
            .padding(.horizontal, prominent ? 18 : 10)
            .frame(minHeight: prominent ? 30 : 24)
            .background(prominent ? amber : inputSurface, in: RoundedRectangle(cornerRadius: 5))
            .overlay {
                RoundedRectangle(cornerRadius: 5).strokeBorder(.white.opacity(prominent ? 0 : 0.16))
                    .allowsHitTesting(false)
            }
            .opacity(enabled ? (configuration.isPressed ? 0.7 : 1) : 0.43)
    }
}

private struct EditorCheckbox: ToggleStyle {
    @Environment(\.isEnabled) private var enabled

    func makeBody(configuration: Configuration) -> some View {
        Button {
            configuration.isOn.toggle()
        } label: {
            HStack(spacing: 8) {
                RoundedRectangle(cornerRadius: 4)
                    .fill(configuration.isOn ? amber : .clear)
                    .overlay {
                        RoundedRectangle(cornerRadius: 4)
                            .strokeBorder(.white.opacity(configuration.isOn ? 0 : 0.4))
                    }
                    .overlay {
                        if configuration.isOn {
                            Image(systemName: "checkmark")
                                .font(.system(size: 9, weight: .bold))
                                .foregroundStyle(.black)
                        }
                    }
                    .frame(width: 15, height: 15)
                    .accessibilityHidden(true)
                configuration.label
            }
            .contentShape(Rectangle())
            .opacity(enabled ? 1 : 0.36)
        }
        .buttonStyle(.plain)
        .accessibilityRepresentation {
            Toggle(isOn: configuration.$isOn) { configuration.label }
                .toggleStyle(.checkbox)
        }
    }
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
    @State private var showSaves = false
    @State private var showADR = false
    @State private var showGarages = false
    @State private var garagePurchases: Set<String> = []
    @State private var visitCities = false
    @State private var discoverDealers = false
    @State private var academy = false
    @State private var academyComplete = false
    @State private var status = ""
    @State private var failed = false
    @State private var backups: [String] = []
    @State private var didSave = false
    @State private var busy = false

    var body: some View {
        VStack(spacing: 0) {
            VStack(alignment: .leading, spacing: 15) {
                saveSelector
                HStack(alignment: .top, spacing: 36) {
                    VStack(alignment: .leading, spacing: 14) {
                        careerFields
                        skillFields
                    }
                    .frame(width: 375, alignment: .leading)
                    .disabled(save == nil || busy)
                    VStack(alignment: .leading, spacing: 16) {
                        truckOptions
                        worldOptions
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
                SteamOptions()
                if !status.isEmpty {
                    Text(status).font(.caption)
                        .foregroundStyle(failed ? Color(nsColor: .systemRed) : .secondary)
                        .fixedSize(horizontal: false, vertical: true)
                }
            }
            .padding(.horizontal, 21)
            .padding(.vertical, 14)
            footer
        }
        .frame(width: 758)
        .background(graphite)
        .toolbar { editorToolbar }
        .preferredColorScheme(.dark)
        .tint(amber)
        .task { if save == nil && status.isEmpty { load(["read"]) } }
    }

    @ToolbarContentBuilder
    private var editorToolbar: some ToolbarContent {
        if #available(macOS 26, *) {
            toolbarTitle.sharedBackgroundVisibility(.hidden)
            ToolbarSpacer(.flexible)
            toolbarUpdate.sharedBackgroundVisibility(.hidden)
        } else {
            toolbarTitle
            toolbarUpdate
        }
    }

    private var toolbarTitle: some ToolbarContent {
        ToolbarItem(placement: .navigation) {
            HStack(spacing: 8) {
                Image(nsImage: NSApp.applicationIconImage)
                    .resizable()
                    .frame(width: 24, height: 24)
                    .accessibilityHidden(true)
                Text("ETS2 Save Editor").font(.system(size: 16, weight: .bold))
            }
        }
    }

    private var toolbarUpdate: some ToolbarContent {
        ToolbarItem(placement: .primaryAction) {
            Button(tr("Check for Updates…"), action: checkForUpdates)
                .buttonStyle(.plain)
                .foregroundStyle(.secondary)
        }
    }

    private var saveSelector: some View {
        VStack(alignment: .leading, spacing: 5) {
            Text(saveSource)
                .font(.system(size: 12, weight: .medium))
                .foregroundStyle(.secondary)
            HStack(spacing: 9) {
                Button {
                    showSaves.toggle()
                } label: {
                    menuLabel(saveLabel)
                }
                .buttonStyle(.plain)
                .accessibilityLabel(tr("Save slot"))
                .accessibilityValue(saveLabel)
                .help(save?.path ?? tr("No save selected"))
                .popover(isPresented: $showSaves, arrowEdge: .bottom) {
                    saveChoices
                }
                Button(tr("Choose…"), action: chooseSave)
                    .buttonStyle(EditorButton())
            }
        }
        .disabled(busy)
    }

    private var saveChoices: some View {
        VStack(alignment: .leading, spacing: 8) {
            Button(tr("Latest Save")) {
                showSaves = false
                load(["read"])
            }
            .buttonStyle(EditorButton())
            if let save, !save.saves.isEmpty {
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 2) {
                        ForEach(save.saves) { choice in
                            Button {
                                showSaves = false
                                load(["read", choice.path])
                            } label: {
                                Text(choice.label)
                                    .frame(maxWidth: .infinity, alignment: .leading)
                                    .padding(.vertical, 5)
                                    .padding(.horizontal, 6)
                                    .contentShape(Rectangle())
                            }
                            .buttonStyle(.plain)
                            .disabled(choice.path == save.path)
                        }
                    }
                }
                .frame(height: min(CGFloat(save.saves.count) * 28, 226))
            }
        }
        .padding(12)
        .frame(width: 385, alignment: .leading)
    }

    private var saveSource: String {
        guard let save else { return tr("Save slot") }
        return save.path.contains("/227300/remote/") ? "Steam Cloud" : tr("Local save")
    }

    private var careerFields: some View {
        VStack(alignment: .leading, spacing: 6) {
            sectionTitle(tr("Career"))
            HStack(alignment: .top, spacing: 16) {
                VStack(alignment: .leading, spacing: 4) {
                    Text(tr("Balance")).foregroundStyle(.secondary)
                    numericField(tr("Balance"), text: $money, prefix: "€", step: stepMoney)
                }
                .frame(maxWidth: .infinity)
                VStack(alignment: .leading, spacing: 4) {
                    Menu {
                        Picker(tr("Career"), selection: $careerMode) {
                            Text(tr("Level")).tag("level")
                            Text("XP").tag("xp")
                        }
                    } label: {
                        HStack(spacing: 4) {
                            Text(careerLabel)
                            Image(systemName: "chevron.down").font(.system(size: 8, weight: .semibold))
                        }
                        .foregroundStyle(.secondary)
                    }
                    .menuStyle(.borderlessButton)
                    .menuIndicator(.hidden)
                    .tint(Color.secondary)
                    .fixedSize()
                    .onChange(of: careerMode) { mode in
                        if let save { career = String(mode == "level" ? save.level : save.xp) }
                    }
                    numericField(careerLabel, text: $career, step: stepCareer)
                }
                .frame(width: 150)
            }
        }
    }

    private var careerLabel: String { careerMode == "level" ? tr("Level") : "XP" }

    private func numericField(_ title: String, text: Binding<String>, prefix: String? = nil, step: @escaping (Int) -> Void) -> some View {
        HStack(spacing: 6) {
            if let prefix { Text(prefix).foregroundStyle(.secondary) }
            TextField(title, text: text)
                .textFieldStyle(.plain)
                .labelsHidden()
                .font(.system(size: 15, design: .monospaced))
            Stepper(title, onIncrement: { step(1) }, onDecrement: { step(-1) })
                .labelsHidden()
                .controlSize(.small)
                .accessibilityLabel(title)
        }
        .padding(.horizontal, 8)
        .frame(height: 28)
        .controlSurface()
    }

    private func stepMoney(_ direction: Int) {
        let amount = Int64(money) ?? 0
        let (value, overflow) = amount.addingReportingOverflow(Int64(direction) * 1000)
        if !overflow { money = String(value) }
    }

    private func stepCareer(_ direction: Int) {
        let highest = careerMode == "level" ? 315821 : 2147483647
        career = String(min(highest, max(0, (careerTarget ?? 0) + direction)))
    }

    private var skillFields: some View {
        VStack(alignment: .leading, spacing: 6) {
            sectionTitle(tr("Skills"))
            Grid(alignment: .leading, horizontalSpacing: 14, verticalSpacing: 5) {
                GridRow {
                    Text("ADR").frame(maxWidth: .infinity, alignment: .leading)
                    Button {
                        showADR.toggle()
                    } label: {
                        menuLabel("\(skills[0].nonzeroBitCount) / 6", monospaced: true)
                    }
                    .buttonStyle(.plain)
                    .frame(width: 113)
                    .accessibilityLabel("ADR")
                    .accessibilityValue(tr("%lld / 6 classes", skills[0].nonzeroBitCount))
                    .popover(isPresented: $showADR, arrowEdge: .bottom) {
                        VStack(alignment: .leading, spacing: 8) {
                            ForEach(classes.indices, id: \.self) { index in
                                Toggle(classes[index], isOn: adrClass(index))
                            }
                        }
                        .toggleStyle(EditorCheckbox())
                        .padding(14)
                    }
                    Button(tr("Max")) { skills[0] = 63 }
                        .accessibilityLabel(tr("Unlock all ADR classes"))
                        .disabled(skills[0] == 63)
                }
                ForEach(1..<names.count, id: \.self) { index in
                    GridRow {
                        Text(names[index])
                            .frame(maxWidth: .infinity, alignment: .leading)
                        HStack(spacing: 6) {
                            Text("\(skills[index]) / 6").font(.system(size: 13, design: .monospaced))
                            Spacer(minLength: 0)
                            Stepper(names[index], value: $skills[index], in: 0...6)
                                .labelsHidden()
                                .controlSize(.small)
                                .accessibilityLabel(names[index])
                        }
                        .padding(.horizontal, 8)
                        .frame(width: 113, height: 24)
                        .controlSurface()
                        Button(tr("Max")) { skills[index] = 6 }
                            .accessibilityLabel(tr("Max out %@", names[index]))
                            .disabled(skills[index] == 6)
                    }
                }
            }
            .buttonStyle(EditorButton())
            .frame(maxWidth: .infinity, alignment: .leading)
        }
    }

    private func menuLabel(_ title: String, monospaced: Bool = false) -> some View {
        HStack(spacing: 5) {
            Text(title)
                .font(.system(size: 13, design: monospaced ? .monospaced : .default))
                .lineLimit(1)
                .truncationMode(.middle)
            Spacer(minLength: 0)
            Image(systemName: "chevron.down")
                .font(.system(size: 9, weight: .semibold))
                .foregroundStyle(.secondary)
        }
        .padding(.horizontal, 8)
        .frame(height: monospaced ? 24 : 27)
        .controlSurface()
    }

    private func sectionTitle(_ title: String) -> some View {
        Text(title).font(.system(size: 17, weight: .bold))
            .accessibilityAddTraits(.isHeader)
    }

    private var truckOptions: some View {
        VStack(alignment: .leading, spacing: 8) {
            sectionTitle(tr("Truck"))
            Toggle(tr("Cancel Quick Job without a fee"), isOn: $cancelJob)
                .help(save?.job ?? "")
                .disabled(save?.canCancel != true || repair || refuel)
            Toggle(tr("Repair truck and trailer"), isOn: $repair)
                .help(tr("Clears damage and permanent wear."))
                .disabled(save?.canRepair != true || cancelJob)
            Toggle(tr("Refuel truck"), isOn: $refuel)
                .disabled(save?.canRefuel != true || cancelJob)
            if let save, !save.canCancel, !save.job.isEmpty {
                Text(tr("No suitable save from before this job was found."))
                    .font(.caption).foregroundStyle(.secondary)
            }
        }
        .toggleStyle(EditorCheckbox())
        .disabled(busy)
    }

    private var availableGarages: Set<String> {
        Set(save?.garages.filter(\.canBuy).map(\.id) ?? [])
    }

    private var allGarages: Binding<Bool> {
        Binding(
            get: { !availableGarages.isEmpty && garagePurchases == availableGarages },
            set: { garagePurchases = $0 ? availableGarages : [] }
        )
    }

    private var worldOptions: some View {
        VStack(alignment: .leading, spacing: 8) {
            sectionTitle(tr("World & Academy"))
            if save?.citiesComplete == true {
                completedOption("Quick travel cities unlocked")
            } else {
                Toggle(tr("Unlock quick travel cities"), isOn: $visitCities)
                    .help(tr("Unlock cities for quick travel (%lld)", save?.unvisitedCities ?? 0))
                    .disabled((save?.unvisitedCities ?? 0) == 0)
            }
            if save?.dealersComplete == true {
                completedOption("Truck dealerships discovered")
            } else {
                Toggle(tr("Discover truck dealerships"), isOn: $discoverDealers)
                    .help(tr("Discover truck dealerships (%lld)", save?.undiscoveredDealers ?? 0))
                    .disabled((save?.undiscoveredDealers ?? 0) == 0)
            }
            HStack(spacing: 6) {
                if availableGarages.isEmpty && save?.garages.contains(where: { $0.isOwned }) == true {
                    completedOption("All available garages owned")
                } else {
                    Toggle(tr("Buy all available garages"), isOn: allGarages)
                        .disabled(availableGarages.isEmpty)
                }
                Spacer(minLength: 0)
                if !garagePurchases.isEmpty && !allGarages.wrappedValue {
                    Text(tr("%lld selected", garagePurchases.count))
                        .font(.caption).foregroundStyle(.secondary)
                }
                Button {
                    showGarages.toggle()
                } label: {
                    Image(systemName: showGarages ? "chevron.down" : "chevron.right")
                        .font(.system(size: 10, weight: .semibold))
                        .frame(width: 22, height: 20)
                        .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .accessibilityLabel(tr("Choose garages"))
                .help(tr("Choose garages"))
                .disabled(save?.garages.isEmpty != false)
                .popover(isPresented: $showGarages, arrowEdge: .trailing) {
                    if let save {
                        GarageOptions(garages: save.garages, selected: $garagePurchases)
                    }
                }
            }
            if academyComplete {
                completedOption("Driving Academy complete")
            } else {
                Toggle(tr("Complete Driving Academy"), isOn: $academy)
                    .help(tr("Completes the three Academy modules for this profile."))
                    .disabled(save == nil)
            }
        }
        .toggleStyle(EditorCheckbox())
        .disabled(busy)
    }

    private func completedOption(_ title: String) -> some View {
        Label(tr(title), systemImage: "checkmark.square.fill")
            .foregroundStyle(.secondary)
    }

    private var footer: some View {
        HStack(spacing: 10) {
            if let version = Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String {
                Text("v\(version) ·")
            }
            Text(verbatim: "Made with <3 by code-a-man ·")
            Button("GitHub") {
                NSWorkspace.shared.open(URL(string: "https://github.com/code-a-man/ets2-save-editor")!)
            }
            .buttonStyle(.plain)
            .help(tr("Open the GitHub repository"))
            Spacer()
            if busy {
                ProgressView().controlSize(.small)
                    .accessibilityLabel(tr("Processing save"))
            }
            if !backups.isEmpty {
                Button(backups.count == 1 ? tr("Show Backup") : tr("Show Backups")) {
                    NSWorkspace.shared.activateFileViewerSelecting(backups.map { URL(fileURLWithPath: $0) })
                }
                .buttonStyle(.plain)
            }
            if hasPendingChanges && !busy {
                Text(tr("Unsaved changes"))
            }
            if didSave && !hasPendingChanges {
                Button(tr("Launch Game"), action: launchGame)
                    .buttonStyle(EditorButton(prominent: true))
                    .disabled(busy)
            } else {
                Button(tr("Save"), action: saveChanges)
                    .keyboardShortcut("s", modifiers: .command)
                    .buttonStyle(EditorButton(prominent: true))
                    .disabled(!canSave || busy)
            }
        }
        .foregroundStyle(.secondary)
        .font(.system(size: 12))
        .padding(.horizontal, 21)
        .padding(.vertical, 11)
        .background(titleSurface)
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

    private var hasPendingChanges: Bool { hasSlotChanges || academy }

    private var hasSlotChanges: Bool {
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

    private func saveChanges() {
        guard let save, canSave, !busy, let amount = Int64(money), let target = careerTarget else { return }
        let writeCareer = hasSlotChanges
        let writeAcademy = academy
        var arguments = ["write", save.path, save.token, String(amount)]
        arguments.append(contentsOf: skills.map(String.init))
        arguments.append(contentsOf: [
            cancelJob ? "cancel" : "keep", "\(careerMode):\(target)", String(repair), String(refuel)
        ])
        if visitCities { arguments.append("visit-cities") }
        if discoverDealers { arguments.append("discover-dealers") }
        arguments.append(contentsOf: garagePurchases.sorted())

        busy = true
        failed = false
        status = ""
        Task { @MainActor in
            defer { busy = false }
            var slotSaved = false
            do {
                if writeCareer {
                    let result = try await fetchSave(arguments)
                    applySave(result)
                    backups = result.backup.map { [$0] } ?? []
                    slotSaved = true
                    didSave = true
                }
                if writeAcademy && !academyComplete {
                    let result = try await Task.detached(priority: .userInitiated) {
                        try runEditor(["academy", save.path], as: AcademyResult.self)
                    }.value
                    if let backup = result.backup {
                        backups.append(backup)
                    }
                    academy = false
                    academyComplete = true
                    if result.added > 0 {
                        status = tr("Driving Academy updated.")
                        didSave = true
                    }
                }
                if slotSaved && status.isEmpty {
                    status = tr("Saved. Load this save in ETS2.")
                }
            } catch {
                status = slotSaved
                    ? tr("Save edits applied. Driving Academy is still pending. %@", error.localizedDescription)
                    : error.localizedDescription
                failed = true
            }
        }
    }

    private func load(_ arguments: [String]) {
        guard !busy else { return }
        busy = true
        status = ""
        failed = false
        Task { @MainActor in
            defer { busy = false }
            do {
                let result = try await fetchSave(arguments)
                applySave(result)
                academy = false
                showGarages = false
                backups = []
                didSave = false
                if result.gameOpen {
                    status = tr("Close ETS2 and click Latest Save before saving.")
                }
            } catch {
                status = error.localizedDescription
                failed = true
            }
        }
    }

    private func fetchSave(_ arguments: [String]) async throws -> SaveState {
        let result = try await Task.detached(priority: .userInitiated) {
            try runEditor(arguments, as: SaveState.self)
        }.value
        guard result.skills.count == 6 else {
            throw EditorError(error: tr("Could not read the skills."))
        }
        return result
    }

    private func applySave(_ result: SaveState) {
        save = result
        academyComplete = result.academyComplete
        if academyComplete { academy = false }
        money = result.money
        skills = result.skills
        cancelJob = false
        garagePurchases.removeAll()
        visitCities = false
        discoverDealers = false
        repair = false
        refuel = false
        career = String(careerMode == "level" ? result.level : result.xp)
    }

}

private struct GarageOptions: View {
    let garages: [GarageChoice]
    @Binding var selected: Set<String>
    @State private var search = ""

    private var visible: [GarageChoice] {
        search.isEmpty ? garages : garages.filter { $0.city.localizedStandardContains(search) }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(tr("Garages")).font(.headline)
            Text(tr("Save to open the selected garages with five slots each at no cost."))
                .font(.callout).foregroundStyle(.secondary)
            TextField(tr("Search cities"), text: $search).textFieldStyle(.roundedBorder)
            List(visible) { garage in
                HStack {
                    if garage.canBuy {
                        Toggle(garage.city.capitalized, isOn: selection(for: garage))
                            .toggleStyle(EditorCheckbox())
                    } else {
                        Text(garage.city.capitalized)
                    }
                    Spacer()
                    Text(garage.label).foregroundStyle(.secondary)
                }
            }
            .frame(height: 270)
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
    @State private var steam: SteamState?
    @State private var busy = false
    @State private var error = ""

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("Steam").font(.system(size: 17, weight: .bold))
                .accessibilityAddTraits(.isHeader)
            HStack(spacing: 8) {
                Toggle("Vulkan + SDR", isOn: Binding(
                    get: { steam?.enabled ?? false },
                    set: { enabled in
                        guard let steam else { return }
                        update([enabled ? "on" : "off", steam.token])
                    }
                ))
                .toggleStyle(EditorCheckbox())
                .disabled(steam == nil || steam?.running == true || busy)
                .help("-color_mode sdr -rdevice vk")
                Text(tr("For potentially higher FPS"))
                    .font(.caption).foregroundStyle(.secondary)
                if busy {
                    ProgressView().controlSize(.small)
                        .accessibilityLabel(tr("Reading Steam settings"))
                }
            }
            if steam?.enabled == true {
                Text(tr("Vulkan can show a black screen at 100% scaling. Choose a different scaling value in ETS2."))
                    .font(.caption).foregroundStyle(.secondary)
                    .padding(.leading, 19)
            }
            if steam?.running == true || !error.isEmpty {
                HStack(spacing: 8) {
                    if steam?.running == true {
                        Text(tr("Quit Steam completely and click Refresh to change this setting."))
                            .font(.caption).foregroundStyle(.secondary)
                    }
                    Button(tr("Refresh")) { update([]) }.buttonStyle(EditorButton()).disabled(busy)
                }
            }
            if !error.isEmpty {
                Text(error).font(.caption).foregroundStyle(Color(nsColor: .systemRed))
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
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
        .windowToolbarStyle(.unifiedCompact(showsTitle: false))
        .windowResizability(.contentSize)
        .commands {
            CommandGroup(replacing: .appInfo) {
                Button(tr("Check for Updates…")) { updater.checkForUpdates(nil) }
            }
        }
    }
}
