<p align="center">
  <img src="cmd/pdf-unseal/assets/icon-144.png" width="96" height="96" alt="">
</p>

<h1 align="center">PDF Unseal</h1>

<p align="center">Drop a PDF, enter its password, get an unlocked copy.</p>

PDF Unseal is a small desktop app that removes the open password and the print, copy and edit restrictions from a PDF. It never changes the original file: it saves an unlocked copy next to it.

## Features

- **Drag and drop**, or click the drop zone to choose a file.
- **Open passwords:** asks for the password once, then saves a copy that opens without one.
- **Restrictions only:** PDFs that open freely but block printing, copying or editing are unlocked without asking anything.
- **Safe saving:** suggests `name - Unsealed.pdf` in the same folder and asks before replacing an existing file.
- **English and Italian**, chosen from your Windows display language.

PDF Unseal only removes protections you are entitled to remove: a PDF with an open password still needs that password.

## Download

Get `pdf-unseal-<version>.exe` from the [latest release](https://github.com/Monska85/PDF-Unseal/releases/latest). It is a single file with nothing to install.

The app is not code-signed yet, so Windows SmartScreen may warn on first launch. Choose **More info**, then **Run anyway**.

Linux and macOS versions are planned.

## Usage

1. Start the downloaded `.exe`.
2. Drop a PDF on the window, or click the drop zone, or use **File → Open** (`Ctrl+O`).
3. If the PDF has an open password, enter it. The eye button shows what you typed.
4. Choose where to save the unlocked copy.

The window then shows the saved file, with a link to open its folder.

### Language

The app follows your Windows display language and falls back to English. To force a language, set `PDF_UNSEAL_LANG`, for example in PowerShell:

```powershell
$env:PDF_UNSEAL_LANG = "it"; .\pdf-unseal-<version>.exe
```

Supported values: `en`, `it`.

## Building from source

Requires Go (see `go.mod` for the version) and, for the tests, [qpdf](https://qpdf.sourceforge.io/).

```sh
go test ./...
GOOS=windows GOARCH=amd64 go build -ldflags="-H windowsgui" -o pdf-unseal.exe ./cmd/pdf-unseal
```

A local build has no embedded manifest, so copy `cmd/pdf-unseal/pdf-unseal.exe.manifest` next to the exe before running it. Release builds embed the manifest, icon and version information, along with the third-party license notices.

## Releasing

Push a tag such as `v1.0.0`. GitHub Actions runs GoReleaser, which publishes `pdf-unseal-<version>.exe`, `THIRD_PARTY_LICENSES.txt` and `checksums.txt` to a GitHub release. A second job wraps that exe in `pdf-unseal-<version>.msix` and attaches it to the same release, ready to upload to the Microsoft Store in Partner Center. The Store signs the package; it is not meant for direct installation. The Store only accepts versions 1.0.0 and later without suffixes, so tags such as `v0.2.0`, `v1.2.0-rc1` or `v1.2.0+build1` get no package.

## Translations

Translations live in `internal/i18n/locales`, one JSON file per language. To add one, copy `en.json` to `<code>.json` and translate the values; the tests check that every file has the same keys.

## Privacy

PDF Unseal works offline and collects no data. See the [privacy policy](PRIVACY.md).

## License

[MIT](LICENSE) © Daniele M.

PDF Unseal is built with [pdfcpu](https://github.com/pdfcpu/pdfcpu) and [walk](https://github.com/lxn/walk). Their licenses, and those of the other bundled libraries, are shown under **Help → About** and published with each release.
