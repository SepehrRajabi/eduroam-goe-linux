// eduroam-goe-linux — a modified Go port of the eduroam CAT Linux installer.
//
// Based on the eduroam CAT Linux installer (https://github.com/GEANT/CAT),
// developed on behalf of the GÉANT project; copyright GEANT Association /
// GEANT Limited. Original authors: Tomasz Wolniewicz, Michał Gasewicz and
// contributors (see NOTICE).
//
// Modified by Sepehr Rajabi, 2026: ported to Go, with a Bubble Tea terminal
// UI and pluggable network backends. This is not the Original Work.
//
// Licensed under the GÉANT Standard Open Source Software Outward Licence.
// See the LICENSE and NOTICE files for the full licence text and the retained
// copyright and Attribution Notices.

package profile

// User-facing messages (English; the CAT generator would normally inject
// translated strings here).
const (
	MsgQuit                = "Really quit?"
	MsgCredentialsPrompt   = "Please enter your credentials:"
	MsgUsernamePrompt      = "enter your userid"
	MsgEnterPassword       = "enter password"
	MsgEnterImportPassword = "enter your import password"
	MsgIncorrectPassword   = "incorrect password"
	MsgRepeatPassword      = "repeat your password"
	MsgPasswordsDiffer     = "passwords do not match"
	MsgEmptyField          = "one of the fields was empty"
	MsgInstallFinished     = "Installation successful"
	MsgCatDirExists        = "Directory %s exists; some of its files may be overwritten."
	MsgContinue            = "Continue?"
	MsgNMNotSupported      = "This NetworkManager version is not supported"
	MsgCertError           = "Certificate file not found, looks like a CAT error"
	MsgDBusError           = "DBus connection problem, a sudo might help"
	MsgP12Title            = "personal certificate file (p12 or pfx)"
	MsgPlaintextWarning    = "This backend writes a configuration file. Be warned that your " +
		"connection password will be saved in this file as clear text."
	MsgWrongUsernameFormat = "Error: Your username must be of the form 'xxx@institutionID' e.g. 'john@example.net'!"
	MsgWrongRealm          = "Error: your username must be in the form of 'xxx@%s'. Please enter the username in the correct format."
	MsgWrongRealmSuffix    = "Error: your username must be in the form of 'xxx@institutionID' and end with '%s'. " +
		"Please enter the username in the correct format."
	MsgUserCertMissing = "personal certificate file not found"
	MsgWired           = "Configure eduroam also on the wired Ethernet?\n\nPlease be aware that doing so will " +
		"introduce a connection delay lasting up to 30 sec. when you connect to an open wired network."
)
