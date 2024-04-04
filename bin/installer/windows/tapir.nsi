!define PRODUCT_VERSION "0.3.7.0"
!define PUBLISHER "Forest Soft"
!define PRODUCT_URL "https://github.com/Forest33/tapir"

VIProductVersion "${PRODUCT_VERSION}"
VIFileVersion "${PRODUCT_VERSION}"
VIAddVersionKey FileVersion "${PRODUCT_VERSION}"
VIAddVersionKey FileDescription "TapirVPN client"
VIAddVersionKey LegalCopyright ""

VIAddVersionKey ProductName "Tapir"
VIAddVersionKey Comments "Installs TapirVPN."
VIAddVersionKey CompanyName "${PUBLISHER}"
VIAddVersionKey ProductVersion "${PRODUCT_VERSION}"
VIAddVersionKey InternalName "Tapir"
;VIAddVersionKey LegalTrademarks " "
;VIAddVersionKey PrivateBuild ""
;VIAddVersionKey SpecialBuild ""

!include "MUI.nsh"
!define MUI_ICON "app.ico"
!define MUI_UNICON "app.ico"

# The name of the installer
Name "Tapir ${PRODUCT_VERSION}"

# The file to write
OutFile "Tapir-${PRODUCT_VERSION}-windows-x86-64.exe"

; Build Unicode installer
Unicode True

# The default installation directory
InstallDir $PROGRAMFILES\Tapir\

; -------
; Registry key to check for directory (so if you install again, it will
; overwrite the old one automatically)
InstallDirRegKey HKLM "Software\Tapir" "Install_Dir"
; -------

# The text to prompt the user to enter a directory
DirText "This will install TapirVPN on your computer. Choose a directory"

!insertmacro MUI_LANGUAGE "English"

#--------------------------------

# The stuff to install
Section "" #No components page, name is not important

# Set output path to the application directory.
CreateDirectory "$APPDATA\Tapir"
SetOutPath "$APPDATA\Tapir"
File wintun.dll

# Set output path to the installation directory.
SetOutPath $INSTDIR

# Put a file there
File Tapir.exe
File app.ico

# Tell the compiler to write an uninstaller and to look for a "Uninstall" section
WriteUninstaller $INSTDIR\Uninstall.exe

CreateDirectory "$SMPROGRAMS\Tapir"
CreateShortCut "$SMPROGRAMS\Tapir\Tapir.lnk" "$INSTDIR\Tapir.exe"
CreateShortCut "$SMPROGRAMS\Tapir\Uninstall.lnk" "$INSTDIR\Uninstall.exe"

; -------
; Write the installation path into the registry
WriteRegStr HKLM SOFTWARE\Tapir "Install_Dir" "$INSTDIR"

; Write the uninstall keys for Windows
WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\Tapir" "DisplayName" "Tapir"
WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\Tapir" "UninstallString" '"$INSTDIR\uninstall.exe"'
WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\Tapir" "Version" "${PRODUCT_VERSION}"
WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\Tapir" "DisplayVersion" "${PRODUCT_VERSION}"
WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\Tapir" "Publisher" "${PUBLISHER}"
WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\Tapir" "DisplayIcon" "$INSTDIR\app.ico"
WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\Tapir" "HelpLink" "${PRODUCT_URL}"
WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\Tapir" "URLUpdateInfo" "${PRODUCT_URL}"
WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\Tapir" "NoModify" 1
WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\Tapir" "NoRepair" 1

WriteUninstaller "$INSTDIR\uninstall.exe"
; -------

SectionEnd # end the section

# The uninstall section
Section "Uninstall"

; -------
; Remove registry keys
DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\Tapir"
DeleteRegKey HKLM SOFTWARE\Tapir
; -------

Delete $SYSDIR\wintun.dll
RMDir /r "$SMPROGRAMS\Tapir"
RMDir /r "$PROFILE\Tapir"
RMDir /r "$APPDATA\Tapir"
RMDir /r $INSTDIR

SectionEnd
