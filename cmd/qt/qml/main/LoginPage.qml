import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls.Material 2.12
import QtQml 2.13
import csc 1.2 as CSC

Page {
	id: page
	height: content.height + 2 * CSC.Style.padding
	implicitHeight: height
	implicitWidth: content.width + 2 * CSC.Style.padding
	Material.accent: CSC.Style.primaryColor

	Component.onCompleted: implicitHeight = content.height + 2 * CSC.Style.padding

	property bool loggedIn: false
	property real formHeight: 0

	Keys.onReturnPressed: continueButton.clicked() // Enter key
	Keys.onEnterPressed: continueButton.clicked()  // Numpad enter key

	Column {
		id: content
		spacing: CSC.Style.padding
		height: childrenRect.height + topPadding
		width: childrenRect.width + leftPadding
		topPadding: 2 * CSC.Style.padding
		leftPadding: 2 * CSC.Style.padding

		Label {
			text: "<h1>Log in to Data Gateway</h1>"
			color: CSC.Style.primaryColor
			maximumLineCount: 1
		}

		Label {
			text: "Data Gateway gives you secure access to your data. Please select the services you would like to access."
			wrapMode: Text.Wrap
			width: repositoryList.width
			lineHeight: 1.2
			color: CSC.Style.grey
			font.pixelSize: 14
		}

		ListView {
			id: repositoryList
			model: LoginModel
			spacing: 0.5 * CSC.Style.padding
			boundsBehavior: Flickable.StopAtBounds
			height: contentHeight
			width: 450
			focus: true
			
			property int loading: 0
			property bool success: false
			
			delegate: CSC.Accordion {
				id: accordion
				heading: repository
				width: repositoryList.width
				success: loggedIn
				enabled: !envsMissing
				anchors.horizontalCenter: parent.horizontalCenter

				property bool current: ListView.isCurrentItem

				onLoadingChanged: repositoryList.loading += (loading ? 1 : -1)
				Component.onCompleted: page.formHeight = Math.max(page.formHeight, loader.item.height)

				onSuccessChanged: {
					if (success) {
						repositoryList.success = true
						loader.item.loading = false
					}
				}

				onOpenChanged: {
					if (open && method == LoginMethod.Password) {
						repositoryList.currentIndex = index
					}
				}
 
				onCurrentChanged: {
					if (!current) {
						accordion.hide()
					}
				}

				Connections {
					target: QmlBridge
					onLoginError: {
						if (index == idx) {
							loader.item.loading = false
							popup.errorMessage = message
							popup.open()
						}
					}
				}

				Loader {
					id: loader
					focus: accordion.open && method == LoginMethod.Password
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
			enabled: repositoryList.loading == 0 && repositoryList.success

			onClicked: {
				if (enabled) {
					QmlBridge.initFuse()
					page.loggedIn = true
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
					if (idx == empty.index) {
						loading = false
						popup.errorMessage = repository + " authorization failed"
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
							passwordField.focus = true
							passwordField.selectAll()
						}
					}
				}
			}

			Label {
				text: "Please log in with your CSC credentials"
				topPadding: 10
				color: CSC.Style.grey
				font.pixelSize: 13
			}

			CSC.TextField {
				id: usernameField
				focus: true
				placeholderText: "Username"
				Layout.fillWidth: true
			}

			CSC.TextField {
				id: passwordField
				placeholderText: "Password"
				errorText: "Please enter valid password"
				echoMode: TextInput.Password
				activeFocusOnTab: true
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