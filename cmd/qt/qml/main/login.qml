import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls.Material 2.12
import QtQuick.Window 2.13
import QtQml 2.13
import csc 1.0 as CSC

Window {
	id: loginWindow
	visible: true
	title: "SD-Connect Filesystem"
	minimumWidth: 500
    minimumHeight: 400
	maximumWidth: minimumWidth
    maximumHeight: minimumHeight

	property var component
	property ApplicationWindow homeWindow
	property QtObject obj: CSC.Style

	Material.accent: CSC.Style.primaryColor

	CSC.Popup {
		id: popup
	}

	Connections {
		target: QmlBridge
		onEnvError: {
			itemWrap.enabled = false
			popup.errorTextContent = err
			popup.errorTextClarify = ""
			popup.open()
		}
		onLoginResult: {
			if (!message) {
				popup.errorTextContent = "Could not create main window"
				popup.errorTextClarify = ""
				
				component = Qt.createComponent("mainWindow.qml")
				if (component.status == Component.Ready) {
					homeWindow = component.createObject(loginWindow, {username: usernameField.text})
					if (homeWindow == null) {
						console.log("Error creating main window")
						popup.open()
						return
					}

					loginButton.state = ""
					loginWindow.hide()
					homeWindow.show()
				} else {
					if (component.status == Component.Error) {
						console.log("Error loading component: " + component.errorString());
					}
					popup.open()
				}
				return
			}

			popup.errorTextContent = message
			popup.errorTextClarify = err
			loginButton.state = ""
			passwordField.selectAll()
			passwordField.focus = true
			popup.open()
		}
	}
	
	Item {
		id: itemWrap
		anchors.fill: parent
		Keys.onReturnPressed: loginButton.clicked() // Enter key
    	Keys.onEnterPressed: loginButton.clicked() // Numpad enter key

		RowLayout {
			anchors.fill: parent
			spacing: 0
			
			Rectangle {
				Layout.fillHeight: true
				Layout.preferredWidth: parent.width * 0.4
				color: CSC.Style.primaryColor

				Image {
					source: "qrc:/qml/images/CSC_logo.svg"
					fillMode: Image.PreserveAspectFit
					width: parent.width - CSC.Style.padding
					anchors.centerIn: parent
				}
			}

			ColumnLayout {
				Layout.fillHeight: true
				Layout.margins: CSC.Style.padding
				spacing: 0

				Label {
					text: "<h1>CSC Login</h1>"
					color: CSC.Style.grey
				}

				CSC.TextField {
					id: usernameField
					placeholderText: "Username"
					focus: true
					Layout.alignment: Qt.AlignCenter
    				Layout.fillWidth: true
				}

				CSC.TextField {
					id: passwordField
					placeholderText: "Password"
					echoMode: TextInput.Password
					Layout.alignment: Qt.AlignCenter
    				Layout.fillWidth: true
				}

				CSC.Button {
					id: loginButton
					text: "Login"
					padding: 15
					Layout.alignment: Qt.AlignCenter
					Layout.fillWidth: true
					
					onClicked: login()

					Component.onCompleted: Layout.minimumHeight = implicitHeight

					states: [
                        State {
                            name: "loading"; 
                            PropertyChanges { target: loginButton; text: ""; loading: true }
                            PropertyChanges { target: itemWrap; enabled: false }
                        }
					]
					
					function login() {
						if (usernameField.text != "" && passwordField.text != "") {
							loginButton.state = "loading"
							popup.close()
							QmlBridge.sendLoginRequest(usernameField.text, passwordField.text)
							return
						}

						popup.errorTextContent = "Incorrect username or password"
						if (popup.opened) {
							popup.visible = false
						}
						popup.open()
					}
				}
			}
		}
	}
}
