import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls.Material 2.12
import Qt.labs.qmlmodels 1.0
import QtQuick.Window 2.13
import QtQml 2.13
import csc 1.0 as CSC

Window {
	id: loginWindow
	visible: true
	title: "SDA Filesystem"
	minimumWidth: 600
    minimumHeight: 450
	maximumWidth: minimumWidth
    maximumHeight: minimumHeight

	property var component
	property ApplicationWindow homeWindow

	Material.accent: CSC.Style.primaryColor

	CSC.Popup {
		id: popup
	}

	Connections {
		target: QmlBridge
		onLoginResult: {
			if (!message) {
				popup.errorMessage = "Could not create main window"
				popup.errorTextClarify = ""
				
				component = Qt.createComponent("mainWindow.qml")
				if (component.status == Component.Ready) {
					homeWindow = component.createObject(loginWindow)
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

			popup.errorMessage = message
			popup.errorTextClarify = err
			loginButton.state = ""
			popup.open()
		}
	}
	
	Item {
		id: itemWrap
		anchors.fill: parent
		Keys.onReturnPressed: loginButton.clicked() // Enter key
    	Keys.onEnterPressed: loginButton.clicked() // Numpad enter key

		RowLayout{
			anchors.fill: parent
			spacing: 0

			Rectangle {
				Layout.fillHeight: true
				Layout.preferredWidth: parent.width * 0.33
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
					Layout.bottomMargin: CSC.Style.padding
				}

				Label {
					text: "<h3>Choose repositories:</h3>"
					color: CSC.Style.grey
					maximumLineCount: 1
				}

				ListView {
					id: repositoryList
					model: LoginModel
					clip: true
					boundsBehavior: Flickable.StopAtBounds
					delegate: repositoryComponent
					Layout.fillWidth: true
					Layout.preferredHeight: 200

					ScrollBar.vertical: ScrollBar { }
				}

				CSC.Button {
					id: loginButton
					text: "Login"
					enabled: checkedButtons > 0
					topInset: 5
					bottomInset: 5
					Layout.leftMargin: CSC.Style.padding
					Layout.rightMargin: CSC.Style.padding
					Layout.fillWidth: true

					property int checkedButtons: 0
					
					onClicked: {
						loginButton.state = "loading"
						popup.close()
						QmlBridge.login()
					}

					// Prevents button from shrinking when loading
					Component.onCompleted: Layout.minimumHeight = implicitHeight

					states: [
                        State {
                            name: "loading"; 
                            PropertyChanges { target: loginButton; text: ""; loading: true }
                            PropertyChanges { target: itemWrap; enabled: false }
                        }
					]
				}
			}
		}
	}

	Component {
		id: repositoryComponent

		ColumnLayout {
			width: repositoryList.width
			spacing: 0

			CheckBox {
				id: checkBox
				checked: false
				text: repository
				leftPadding: 15

				background: Rectangle {
					color: "white"
				}

				onCheckedChanged: {
					if (checked) {
						if (method == LoginMethod.Password) {
							usernameField.forceActiveFocus() 
						}
						loginButton.checkedButtons++
					} else {
						loginButton.checkedButtons--
					}
					LoginModel.setChecked(index, checked)
				}
			}

			Frame {
				id: input
				Layout.fillWidth: true
				Layout.rightMargin: CSC.Style.padding
				Layout.leftMargin: CSC.Style.padding
				visible: checkBox.checked && method == LoginMethod.Password
				topPadding: 0

				ColumnLayout {
					width: parent.width
					spacing: 0

					CSC.TextField {
						id: usernameField
						placeholderText: "Username"
						Layout.fillWidth: true

						onTextChanged: LoginModel.setUsername(index, text)
					}

					CSC.TextField {
						id: passwordField
						placeholderText: "Password"
						Layout.fillWidth: true
						echoMode: TextInput.Password

						onTextChanged: LoginModel.setPassword(index, text)
					}
				}

				background: Rectangle {
					color: "transparent"

					Rectangle {
						color: CSC.Style.lightGreyBlue
						height: input.height * 0.6
						width: parent.width
					}

					Rectangle {
						color: CSC.Style.grey
						height: parent.height * 0.6
						width: 1
						anchors.left: parent.left
					}

					Rectangle {
						color: CSC.Style.grey
						height: parent.height * 0.6
						width: 1
						anchors.right: parent.right
					}

					Rectangle {
						color: CSC.Style.lightGreyBlue
						radius: 5
						z: -1
						height: parent.height * 0.6
						width: parent.width
						border.color: CSC.Style.grey
						border.width: 1
						anchors.bottom: parent.bottom
					}
				}
			}
		} 						
	}
}
