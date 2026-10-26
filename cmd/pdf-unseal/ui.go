package main

// ui is the platform-specific part of the application: the window and its dialogs.
type ui interface {
	// ShowError shows an error message.
	ShowError(message string)
	// ShowSaved reports that the unsealed PDF was written to path.
	ShowSaved(path string)
	// AskPassword asks for the password of the named file. ok is false if the user cancels.
	AskPassword(fileName, message string) (password string, ok bool)
	// ConfirmOverwrite asks whether to replace the existing file at path.
	ConfirmOverwrite(path string) bool
	// AskSavePath asks where to save the unsealed PDF, starting from suggested. ok is false if the user cancels.
	AskSavePath(suggested string) (path string, ok bool, err error)
}
