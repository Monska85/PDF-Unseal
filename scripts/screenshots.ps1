# Takes the Microsoft Store screenshots: starts the app in one language, drives it with
# keystrokes through open, password and save, and captures the window at each step.
# Each capture is centered on a 1366x768 canvas, the Store's minimum desktop size.
# Runs on an interactive Windows desktop, such as a GitHub Actions windows runner.
param(
    [Parameter(Mandatory)] [string] $Exe,      # build with the embedded manifest
    [Parameter(Mandatory)] [string] $Pdf,      # sample PDF protected with $Password
    [Parameter(Mandatory)] [string] $Lang,     # en or it
    [Parameter(Mandatory)] [string] $OutDir,
    [string] $Password = 'example'
)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Windows.Forms, System.Drawing

Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
using System.Text;

public static class Win {
    public struct RECT { public int Left, Top, Right, Bottom; }
    delegate bool EnumProc(IntPtr hwnd, IntPtr lParam);

    [DllImport("user32.dll")] static extern bool EnumWindows(EnumProc cb, IntPtr lParam);
    [DllImport("user32.dll")] static extern uint GetWindowThreadProcessId(IntPtr hwnd, out uint pid);
    [DllImport("user32.dll")] static extern bool IsWindowVisible(IntPtr hwnd);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] static extern int GetWindowText(IntPtr hwnd, StringBuilder text, int max);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] static extern int GetClassName(IntPtr hwnd, StringBuilder name, int max);
    [DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
    [DllImport("user32.dll")] static extern bool GetWindowRect(IntPtr hwnd, out RECT rect);
    [DllImport("dwmapi.dll")] static extern int DwmGetWindowAttribute(IntPtr hwnd, int attr, out RECT rect, int size);
    [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hwnd);
    [DllImport("user32.dll")] public static extern bool SetWindowPos(IntPtr hwnd, IntPtr after, int x, int y, int cx, int cy, uint flags);

    // Find returns the first visible top-level window of pid whose title is title, or zero.
    public static IntPtr Find(uint pid, string title) {
        IntPtr found = IntPtr.Zero;
        EnumWindows((h, l) => {
            uint p;
            GetWindowThreadProcessId(h, out p);
            if (p != pid || !IsWindowVisible(h)) return true;
            var sb = new StringBuilder(256);
            GetWindowText(h, sb, sb.Capacity);
            if (sb.ToString() != title) return true;
            found = h;
            return false;
        }, IntPtr.Zero);
        return found;
    }

    // Dialogs returns how many visible dialog boxes (message boxes, file dialogs) pid has open.
    public static int Dialogs(uint pid) {
        int n = 0;
        EnumWindows((h, l) => {
            uint p;
            GetWindowThreadProcessId(h, out p);
            if (p != pid || !IsWindowVisible(h)) return true;
            var sb = new StringBuilder(64);
            GetClassName(h, sb, sb.Capacity);
            if (sb.ToString() == "#32770") n++;
            return true;
        }, IntPtr.Zero);
        return n;
    }

    // Bounds returns the visible frame of hwnd as x, y, width, height, without the invisible resize borders.
    public static int[] Bounds(IntPtr hwnd) {
        RECT r;
        if (DwmGetWindowAttribute(hwnd, 9, out r, Marshal.SizeOf(typeof(RECT))) != 0) GetWindowRect(hwnd, out r);
        return new int[] { r.Left, r.Top, r.Right - r.Left, r.Bottom - r.Top };
    }
}
'@

$titles = @{
    en = @{ main = 'PDF Unseal'; save = 'Save unsealed PDF' }
    it = @{ main = 'PDF Unseal'; save = 'Salva il PDF sbloccato' }
}[$Lang]
if (-not $titles) { throw "Unsupported language '$Lang'" }

New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

function Wait-Window([uint32] $procId, [string] $title) {
    $deadline = (Get-Date).AddSeconds(20)
    while ((Get-Date) -lt $deadline) {
        $h = [Win]::Find($procId, $title)
        if ($h -ne [IntPtr]::Zero) { Start-Sleep -Milliseconds 700; return $h }
        Start-Sleep -Milliseconds 200
    }
    Save-Screen (Join-Path $OutDir "debug-$Lang.png")
    throw "Window '$title' did not appear"
}

function Send([IntPtr] $hwnd, [string] $keys) {
    [void][Win]::SetForegroundWindow($hwnd)
    Start-Sleep -Milliseconds 300
    [System.Windows.Forms.SendKeys]::SendWait($keys)
}

# SendKeys treats + ^ % ~ ( ) { } [ ] as commands; wrap them in braces to type them.
function Escape([string] $text) { [regex]::Replace($text, '[+^%~(){}\[\]]', '{$0}') }

function Save-Screen([string] $path) {
    $b = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds
    $bmp = New-Object System.Drawing.Bitmap $b.Width, $b.Height
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.CopyFromScreen($b.Location, [System.Drawing.Point]::Empty, $b.Size)
    $bmp.Save($path, [System.Drawing.Imaging.ImageFormat]::Png)
    $g.Dispose(); $bmp.Dispose()
}

# Capture captures the main window, including any dialog on top of it, onto a 1366x768 canvas.
function Capture([IntPtr] $hwnd, [string] $name) {
    # Park the mouse on the taskbar, away from the window, so no hover tint or tooltip shows.
    [void][Win]::SetCursorPos(5, 760)
    Start-Sleep -Milliseconds 800
    $b = [Win]::Bounds($hwnd)
    $r = New-Object System.Drawing.Rectangle $b[0], $b[1], $b[2], $b[3]
    $shot = New-Object System.Drawing.Bitmap $r.Width, $r.Height
    $g = [System.Drawing.Graphics]::FromImage($shot)
    $g.CopyFromScreen($r.Location, [System.Drawing.Point]::Empty, $r.Size)
    $g.Dispose()

    $canvas = New-Object System.Drawing.Bitmap 1366, 768
    $g = [System.Drawing.Graphics]::FromImage($canvas)
    $g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
    $bg = New-Object System.Drawing.Drawing2D.LinearGradientBrush ([System.Drawing.Point]::new(0, 0)), ([System.Drawing.Point]::new(0, 768)), ([System.Drawing.Color]::FromArgb(0xE6, 0xEE, 0xF8)), ([System.Drawing.Color]::FromArgb(0xF7, 0xF9, 0xFC))
    $g.FillRectangle($bg, 0, 0, 1366, 768)
    $x = [int]((1366 - $r.Width) / 2); $y = [int]((768 - $r.Height) / 2)
    # A soft shadow so the window stands out from the background.
    for ($i = 12; $i -ge 1; $i--) {
        $shadow = New-Object System.Drawing.SolidBrush ([System.Drawing.Color]::FromArgb(6, 0, 0, 0))
        $g.FillRectangle($shadow, $x - $i, $y - $i + 4, $r.Width + 2 * $i, $r.Height + 2 * $i)
        $shadow.Dispose()
    }
    $g.DrawImageUnscaled($shot, $x, $y)
    $canvas.Save((Join-Path $OutDir "$name.png"), [System.Drawing.Imaging.ImageFormat]::Png)
    $g.Dispose(); $bg.Dispose(); $canvas.Dispose(); $shot.Dispose()
}

# Work on a copy, so the saved "- Unsealed" file lands in a scratch folder.
$work = Join-Path ([IO.Path]::GetTempPath()) ("pdf-unseal-shots-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $work | Out-Null
$inputPdf = Join-Path $work (Split-Path $Pdf -Leaf)
Copy-Item $Pdf $inputPdf

$env:PDF_UNSEAL_LANG = $Lang
$proc = Start-Process -FilePath $Exe -PassThru
try {
    $procId = [uint32]$proc.Id
    $main = Wait-Window $procId $titles.main
    # Larger than the default size so the window fills more of the canvas; the screen is 1024x768.
    [void][Win]::SetWindowPos($main, [IntPtr]::Zero, 62, 40, 900, 660, 0x0040)
    Capture $main '1-drop'

    # File > Open, type the path, confirm.
    Send $main '^o'
    Start-Sleep -Seconds 2
    [System.Windows.Forms.SendKeys]::SendWait((Escape $inputPdf))
    [System.Windows.Forms.SendKeys]::SendWait('{ENTER}')

    # The password dialog is titled with the file name.
    $dialog = Wait-Window $procId (Split-Path $inputPdf -Leaf)
    Send $dialog (Escape $Password)
    # Tab to the eye button and press it, so the screenshot shows the password.
    [System.Windows.Forms.SendKeys]::SendWait('{TAB} ')
    Capture $main '2-password'

    # Unlock, then accept the suggested save path.
    Send $dialog '{ENTER}'
    $save = Wait-Window $procId $titles.save
    # On the Windows Server runner the dialog opens Documents instead of the PDF's folder
    # (a desktop PC shows the PDF's folder), so type the full path of the suggested name.
    $saved = Join-Path $work ([IO.Path]::GetFileNameWithoutExtension($inputPdf) + ' - Unsealed.pdf')
    Send $save '^a'
    [System.Windows.Forms.SendKeys]::SendWait((Escape $saved))
    [System.Windows.Forms.SendKeys]::SendWait('{ENTER}')
    # Wait for the unsealed copy; unsealing runs in the background.
    $deadline = (Get-Date).AddSeconds(20)
    while (-not (Test-Path $saved) -and (Get-Date) -lt $deadline) { Start-Sleep -Milliseconds 250 }
    Start-Sleep -Seconds 1
    if (-not (Test-Path $saved) -or [Win]::Dialogs($procId) -ne 0) {
        Save-Screen (Join-Path $OutDir "debug-$Lang.png")
        $found = Get-ChildItem $env:USERPROFILE -Recurse -Filter '* - Unsealed.pdf' -ErrorAction SilentlyContinue | Select-Object -First 3 -ExpandProperty FullName
        throw "Saving did not finish cleanly: '$saved' exists = $(Test-Path $saved); unsealed files found: $($found -join ', ')"
    }
    Capture $main '3-saved'
} finally {
    if (-not $proc.HasExited) { Stop-Process -Id $proc.Id -Force }
    Remove-Item $work -Recurse -Force -ErrorAction SilentlyContinue
}
