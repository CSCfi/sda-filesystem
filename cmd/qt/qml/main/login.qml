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
	title: "SD-Connect FUSE"
	minimumWidth: 500
    minimumHeight: 400
	maximumWidth: minimumWidth
    maximumHeight: minimumHeight

	property int margins: 20
	property ApplicationWindow homeWindow
	Material.accent: CSC.Style.primaryColor

	CSC.Popup {
		id: popup
	}

	Connections {
		target: QmlBridge
		onEnvError: {
			itemWrap.enabled = false
			popup.errorTextContent = err
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
					width: parent.width - loginWindow.margins
					anchors.centerIn: parent
				}
			}

			ColumnLayout {
				Layout.fillHeight: true
				Layout.margins: loginWindow.margins
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
					
					function login() {
						var loginError = "Incorrect username or password"

						if (usernameField.text != "" && passwordField.text != "") {
							loginError = QmlBridge.sendLoginRequest(usernameField.text, passwordField.text)

							if (!loginError) {
								var component = Qt.createComponent("mainWindow.qml")
								if (component.status != Component.Ready) {
									if (component.status == Component.Error)
										console.log("Error loading component:\n" + component.errorString());
									
									popup.errorTextContent = "Could not create window. Our bad :/"
									popup.open()
									return
								}

								homeWindow = component.createObject(loginWindow, {username: usernameField.text})
								if (homeWindow == null) {
									console.log("Error creating window object")
									popup.errorTextContent = "Could not create window. Our bad :/"
									popup.open()
									return
								}

								loginWindow.hide()
								homeWindow.show()
								return
							}

							passwordField.selectAll()
							passwordField.focus = true
						}

						popup.errorTextContent = loginError
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
