import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls.Material 2.12
import QtQuick.Window 2.13
import csc 1.0 as CSC

ApplicationWindow {
	id: mainWindow
    visible: true
    title: "SD-Connect FUSE"
	width: 1000
	height: 600
	minimumHeight: sideBarView.height + toolbar.height
	minimumWidth: homePage.implicitWidth + sideBarView.width

	property string username
	signal logout()

	Material.primary: CSC.Style.primaryColor

	header: ToolBar {
		id: toolbar

        RowLayout {
            anchors.fill: parent

            Image {
				source: "qrc:/qml/images/CSC_logo.svg"
				fillMode: Image.PreserveAspectFit
				Layout.preferredHeight: logoutButton.height
				Layout.preferredWidth: paintedWidth
				Layout.alignment: Qt.AlignLeft
				Layout.margins: 5
			}

            ToolButton {
				id: logoutButton
                text: username
				font.capitalization: Font.MixedCase
				icon.source: "qrc:/qml/images/chevron-down.svg"
				Layout.alignment: Qt.AlignRight
				LayoutMirroring.enabled: true

				Material.foreground: "white"

				onClicked: menu.open()

				Menu {
					id: menu
					y: logoutButton.height

					MenuItem {
						text: "Logout"
						onTriggered: mainWindow.logout()
					}
				}
            }
        }
    }

	CSC.Popup {
		id: popup
	}

	/*Connections {
		target: QmlBridge
		onMountError: {
			popup.errorTextContent = err
			popup.open()
		}
	}*/

	RowLayout {
		spacing: 0
		anchors.fill: parent

		Rectangle {
			color: CSC.Style.tertiaryColor
			Material.foreground: "white"
			Layout.fillHeight: true
			Layout.preferredWidth: 200

			ListView {
				id: sideBarView
				anchors.verticalCenter: parent.verticalCenter
				anchors.right: parent.right
				anchors.left: parent.left
				height: contentHeight

				model: ListModel {
					ListElement {
						name: "Home"
						image: "qrc:/qml/images/house-door.svg"
					}
					ListElement {
						name: "Logs"
						image: "qrc:/qml/images/layout-text-sidebar-reverse.svg"
					}
				}

				highlight: Rectangle { 
					color: CSC.Style.secondaryColor
				}

				delegate: ItemDelegate {
					text: name
					icon.source: image
					width: parent.width
					highlighted: sideBarView.highlightedIndex === index

					onClicked: {
						if (sideBarView.currentIndex != index) {
							sideBarView.currentIndex = index
						}
					}
				}
			}
		}

		StackLayout {
			id: stack
			currentIndex: sideBarView.currentIndex
			Layout.fillWidth: true
			Layout.fillHeight: true

			HomePage {
				id: homePage
			}

			LogPage {
				id: logPage
			}
		}
	}
}