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
	minimumHeight: content.height + 2 * CSC.Style.padding
	minimumWidth: content.width + 2 * CSC.Style.padding
	maximumHeight: minimumHeight
	maximumWidth: minimumWidth

	property var component
	property ApplicationWindow homeWindow

	Material.accent: CSC.Style.primaryColor

	CSC.Popup {
		id: popup
	}

	Connections {
		target: QmlBridge
		onInitError: {
			content.enabled = false
			popup.errorMessage = message
			popup.errorTextClarify = err
			popup.open()
		}
	}

	Column {
		id: content
		spacing: CSC.Style.padding
		height: childrenRect.height + topPadding
		width: childrenRect.width + leftPadding
		topPadding: 2 * CSC.Style.padding
		leftPadding: 2 * CSC.Style.padding
		
		Timer {
            id: timer
            interval: 0; running: false; repeat: false
            onTriggered: loginWindow.height = content.height + 2 * CSC.Style.padding
        }

		onHeightChanged: timer.restart()

		RowLayout {
			Image {
				source: "qrc:/qml/images/CSC_logo.svg"
				fillMode: Image.PreserveAspectFit
				Layout.preferredWidth: paintedWidth
				Layout.preferredHeight: 40
			}

			Text {
				text: "<h3>Sensitive Data Services</h3>"
				color: CSC.Style.grey
				maximumLineCount: 1
			}
		}

		Label {
			text: "<h1>SDA Filesystem</h1>"
			color: CSC.Style.primaryColor
			maximumLineCount: 1
		}

		Label {
			text: "Select one or more services to connect to"
			color: CSC.Style.grey
			maximumLineCount: 1
		}

		ListView {
			id: repositoryList
			model: LoginModel
			spacing: 0.5 * CSC.Style.padding
			boundsBehavior: Flickable.StopAtBounds
			height: contentHeight
			width: 450
			
			delegate: CSC.Accordion {
				id: accordion
				heading: repository
				width: repositoryList.width
				success: loggedIn
				anchors.horizontalCenter: parent.horizontalCenter

				onSuccessChanged: {
					if (success) {
						continueButton.enabled = true
						loader.item.loading = false
					}
				}

				Connections {
					target: QmlBridge
					onLoginError: {
						if (repository == rep) {
							loader.item.loading = false
							popup.errorMessage = message
							popup.errorTextClarify = err
							popup.open()
						}
					}
				}

				Loader {
					id: loader
					width: parent.width - 2 * CSC.Style.padding
					sourceComponent: (method == LoginMethod.Password) ? passwordComponent : tokenComponent
					anchors.horizontalCenter: parent.horizontalCenter

					onLoaded: {
						loader.item.index = index
						loader.item.repository = repository
						loader.item.open = Qt.binding(function() { return accordion.open })
						accordion.loading = Qt.binding(function() { return loader.item.loading })
					}
				}
			}
		}

		CSC.Button {
			id: continueButton
			text: "Continue"
			enabled: false
			leftPadding: 30
			rightPadding: 30
			
			onClicked: {
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

					loginWindow.hide()
					homeWindow.show()
				} else {
					if (component.status == Component.Error) {
						console.log("Error loading component: " + component.errorString());
					}
					popup.open()
				}
			}
		}
	}

	Component {
		id: tokenComponent

		Item {
			id: empty 

			property int index
			property string repository
			property bool open
			property bool loading: false

			onOpenChanged: {
				loading = true
				QmlBridge.loginWithToken(index)
			}

			Connections {
				target: QmlBridge
				onLogin401: {
					if (repository == empty.repository) {
						loading = false
						popup.errorMessage = "Invalid " + repository + " token"
						popup.errorTextClarify = ""
						popup.open()
					}
				}
			}
		}
	}
	
	Component {
		id: passwordComponent

		ColumnLayout {
			id: form
			width: parent.width
			spacing: 0.5 * CSC.Style.padding

			property int index
			property string repository
			property bool open
			property bool loading: !enabled

			onOpenChanged: {
				if (open) {
					usernameField.forceActiveFocus()
				}
			}

			Keys.onReturnPressed: loginButton.clicked() // Enter key
	    	Keys.onEnterPressed: loginButton.clicked()  // Numpad enter key

			Connections {
				target: QmlBridge
				onLogin401: {
					if (repository == form.repository) {
						passwordField.errorVisible = true
						form.enabled = true

						if (usernameField.text != "") {
							passwordField.selectAll()
							passwordField.forceActiveFocus()
						} else {
							usernameField.forceActiveFocus()
						}
					}
				}
			}

			Text {
				text: "Please log in with your CSC credentials"
				topPadding: 10
			}

			CSC.TextField {
				id: usernameField
				placeholderText: "Username"
				Layout.fillWidth: true
			}

			CSC.TextField {
				id: passwordField
				placeholderText: "Password"
				errorText: "Please enter valid password"
				echoMode: TextInput.Password
				Layout.fillWidth: true
			}

			CSC.Button {
				id: loginButton
				text: "Login"
				outlined: true
				topPadding: 10
				bottomPadding: 10

				onClicked: {
					popup.close()
					form.enabled = false
					passwordField.errorVisible = false
					QmlBridge.loginWithPassword(form.index, usernameField.text, passwordField.text)
				}
			}
		}			
	}
}