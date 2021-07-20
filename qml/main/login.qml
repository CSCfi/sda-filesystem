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
	Material.accent: CSC.Style.primaryColor

	CSC.Popup {
		id: popup
		errorTextContent: ""
	}

	Connections
	{
		target: qmlBridge
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
				}

				CSC.TextField {
					id: passwordField
					placeholderText: "Password"
					echoMode: TextInput.Password
				}

				Button {
					id: loginButton
					text: "<b>Login</b>"
					hoverEnabled: true
					padding: 15
					Layout.alignment: Qt.AlignCenter
					Layout.fillWidth: true
					Material.foreground: loginButton.enabled ? "white" : CSC.Style.disabledForeground

					background: Rectangle {
						radius: 4
						color: loginButton.enabled ? (loginButton.pressed ? "#9BBCB7" : (loginButton.hovered ? "#61958D" : CSC.Style.primaryColor)) : CSC.Style.disabledBackground
					}

					onClicked: login()
					
					function login() {
						var loginError = "Incorrect username or password"

						if (usernameField.text != "" && passwordField.text != "") {
							loginError = qmlBridge.sendLoginRequest(usernameField.text, passwordField.text)
							if (!loginError) {
								var component = Qt.createComponent("home.qml")
								var homewindow = component.createObject(loginWindow, {username: usernameField.text})
								if (homewindow == null) {
									console.log("Error creating home window")
									popup.errorTextContent = "Could not create home window. Our bad :/"
									popup.open()
									return
								}
								loginWindow.hide()
								homewindow.show()
								return
							}
							passwordField.selectAll()
							passwordField.focus = true
						}

						popup.errorTextContent = loginError
						popup.open()
					}
				}

				/*Text {
					text: "<a href='https://my.csc.fi/forgotPassword'>Forgot your password?</a>"
					onLinkActivated: Qt.openUrlExternally(link)
					Layout.alignment: Qt.AlignCenter

					MouseArea {
						anchors.fill: parent
						acceptedButtons: Qt.NoButton 
						cursorShape: parent.hoveredLink ? Qt.PointingHandCursor : Qt.ArrowCursor
					}
				}*/
			}
		}
	}
}

/*Label {
	id: loginError
	text: "Incorrect username or password"
	color: CSC.Style.red
	padding: 10
	visible: false
	Layout.fillWidth: true
	background: Rectangle {
		color: Qt.rgba(CSC.Style.red.r, CSC.Style.red.g, CSC.Style.red.b, 0.3)
		radius: 4

		Image {
			source: "qrc:/qml/images/x-lg.svg"
			opacity: closeLoginError.containsMouse ? 0.7 : 1.0
			height: parent.height / 3
			fillMode: Image.PreserveAspectFit
			anchors.verticalCenter: parent.verticalCenter
			anchors.right: parent.right
			anchors.rightMargin: loginError.padding

			MouseArea {
				id: closeLoginError
				anchors.fill: parent
				hoverEnabled: true
				onClicked: {
					loginError.visible = false
				}
			}
		}
	}
}*/
