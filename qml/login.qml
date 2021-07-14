import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls.Material 2.12
import QtQuick.Window 2.13

Window {
	id: loginWindow
	visible: true
	title: "SD-Connect FUSE"
	minimumWidth: 500
    minimumHeight: 400

	property int margins: 20
	property color primaryColor: "#2B6676"

	Item {
		focus: true
		anchors.fill: parent
		Keys.onReturnPressed: loginButton.login() // Enter key
    	Keys.onEnterPressed: loginButton.login() // Numpad enter key

		RowLayout {
			anchors.fill: parent
			spacing: 0
			
			Rectangle {
				Layout.fillHeight: true
				Layout.preferredWidth: parent.width * 0.4
				color: primaryColor

				Image {
					source: "CSC_logo_no_tagline.svg"
					fillMode: Image.PreserveAspectFit
					width: parent.width - loginWindow.margins
					anchors.centerIn: parent
				}
			}

			ColumnLayout {
				Layout.fillHeight: true
				Layout.margins: loginWindow.margins
				spacing: 10

				Text {
					text: "<h1>CSC Login</h1>"
					color: "grey"
				}

				Label {
					id: loginError
					text: "Incorrect username or password"
					color: "#A91C29"
					padding: 10
					visible: false
					Layout.fillWidth: true
					background: Rectangle {
						color: "#4DA91C29"
						radius: 4

						Image {
							source: "x-lg.svg"
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
				}

				TextField {
					id: usernameField
					placeholderText: "Username"
					Layout.alignment: Qt.AlignCenter
					Layout.fillWidth: true
					Material.accent: primaryColor
				}

				TextField {
					id: passwordField
					placeholderText: "Password"
					echoMode: TextInput.Password
					Layout.alignment: Qt.AlignCenter
					Layout.fillWidth: true
					Material.accent: primaryColor
				}

				Button {
					id: loginButton
					text: "<b>Login</b>"
					hoverEnabled: true
					padding: 15
					Layout.alignment: Qt.AlignCenter
					Layout.fillWidth: true
					Material.foreground: "white"

					background: Rectangle {
						radius: 4
						color: loginButton.pressed ? "#9BBCB7" : (loginButton.hovered ? "#61958D" : primaryColor)
					}

					onClicked: loginButton.login()
					
					function login() {
						if (usernameField.text != "" && passwordField.text != "") {
							var loginSuccess = qmlBridge.sendLoginRequest(usernameField.text, passwordField.text)
							if (loginSuccess) {
								var component = Qt.createComponent("home.qml")
								var homewindow = component.createObject(loginWindow)
								if (homewindow == null) {
									console.log("Error creating home window")
									return;
								}
								loginWindow.hide()
								homewindow.show()
								return
							}
						}
						loginError.visible = true
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

/*ApplicationWindow {
    visible: true
    title: "SD-Connect FUSE"

	header: TabBar {
        TabButton {
        	text: qsTr("Home")
			width: implicitWidth
    	}
		TabButton {
        	text: qsTr("Logs")
			width: implicitWidth
    	}
    }
}*/
