import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls.Material 2.12
import QtQml 2.13
import csc 1.0 as CSC

Page {
	id: page
	height: content.height + 2 * CSC.Style.padding
	implicitWidth: content.width + 2 * CSC.Style.padding
	Material.accent: CSC.Style.primaryColor

	Component.onCompleted: implicitHeight = content.height + 2 * CSC.Style.padding

	property bool loggedIn: false

	Column {
		id: content
		spacing: CSC.Style.padding
		height: childrenRect.height + topPadding
		width: childrenRect.width + leftPadding
		topPadding: 2 * CSC.Style.padding
		leftPadding: 2 * CSC.Style.padding

		Label {
			text: "<h1>Data Gateway</h1>"
			color: CSC.Style.primaryColor
			maximumLineCount: 1
		}

		Label {
			text: "Select one or more services to connect to"
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
						if (index == idx) {
							loader.item.loading = false
							popup.errorMessage = message + ". Check logs for further details"
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
			
			onClicked: {
				QmlBridge.initFuse()
				page.loggedIn = true
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
					if (idx == empty.index) {
						loading = false
						popup.errorMessage = "Invalid " + repository + " token"
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
			property bool loading: false

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
					if (idx == form.index) {
						passwordField.errorVisible = true
						form.enabled = true
						form.loading = false

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
				font.pixelSize: 12
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
					form.loading = true
					passwordField.errorVisible = false
					QmlBridge.loginWithPassword(form.index, usernameField.text, passwordField.text)
				}
			}
		}			
	}
}