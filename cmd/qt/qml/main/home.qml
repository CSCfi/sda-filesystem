import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls.Material 2.12
import csc 1.0 as CSC

ApplicationWindow {
	id: homeWindow
    visible: true
    title: "SD-Connect FUSE"
	width: 700 // TODO: modify
	height: 500 // TODO: modify

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
						onTriggered: homeWindow.logout()
					}
				}
            }
        }
    }

	CSC.Popup {
		id: popup
	}

	Drawer {
        id: sideBar

        y: toolbar.height
        width: 200
        height: homeWindow.height - toolbar.height

        modal: false
        interactive: false
        position: 1
        visible: true

        ListView {
            id: sideBarView
            anchors.fill: parent

            model: ListModel {
				ListElement {
					name: "Home"
				}
				ListElement {
					name: "Logs"
				}
			}

            delegate: ItemDelegate {
                text: name
                width: parent.width
            }
        }
    }
	
	GridLayout{
		columns: 4
		rows: 3
		columnSpacing: spacing
		rowSpacing: spacing
		anchors.fill: parent
		anchors.leftMargin: sideBar.width + spacing
		anchors.topMargin: spacing

		property int spacing: 20

		Frame {
			id: acceptFrame
			Layout.fillHeight: true
			Layout.fillWidth: true
			Layout.preferredHeight: Layout.rowSpan
			Layout.preferredWidth: Layout.columnSpan
			Layout.columnSpan: 2
			Layout.rowSpan: 1
			Layout.column: 0
			Layout.row: 0

			Layout.minimumHeight: contentHeight

			background: Rectangle {
				color: CSC.Style.lightGreen
			}

			ColumnLayout {
				anchors.fill: parent

				Text {
					text: "<h3>FUSE will be mounted at:</h3>"
				}

				CSC.TextField {
					id: mountField
					text: QmlBridge.mountPoint
					Layout.fillWidth: true
				}

				CSC.Button {
					id: acceptButton
					text: "Accept"
					outlined: true
					enabled: true
					Layout.alignment: Qt.AlignRight

					/*Connections
					{
						target: QmlBridge
						onMountError: {
							popup.errorTextContent = err
							popup.open()
						}
					}*/ 

					onClicked: {
						if (acceptButton.state == "") {
							QmlBridge.changeMountPoint(mountField.text)
							acceptButton.state = 'accepted'
						} else {
							acceptButton.state = ""
						}
					}

					states: [
						State {
							name: "accepted"
							PropertyChanges { target: acceptButton; text: "Change" }
							PropertyChanges { target: mountField; enabled: false }
							PropertyChanges { target: loadButton; enabled: true }
						}
					]
				}
			}
		}

		Frame {
			Layout.fillHeight: true
			Layout.fillWidth: true
			Layout.preferredHeight: Layout.rowSpan
			Layout.preferredWidth: Layout.columnSpan
			Layout.columnSpan: 2
			Layout.rowSpan: 3
			Layout.column: 2
			Layout.row: 0

			ListView {
				id: projectView
				anchors.fill: parent

				model: ProjectModel
				delegate: Text {
					text: projectName + "  " + containerCount
				}
			}
		}

		CSC.Button {
			id: openButton
			text: "Open FUSE"
			Layout.fillHeight: true
			Layout.fillWidth: true
			Layout.preferredHeight: Layout.rowSpan 
			Layout.preferredWidth: Layout.columnSpan
			Layout.columnSpan: 1
			Layout.rowSpan: 1
			Layout.column: 0
			Layout.row: 1
			enabled: false
			onClicked: QmlBridge.openFuse()
		}

		CSC.Button {
			id: loadButton
			text: "Load FUSE"
			Layout.fillHeight: true
			Layout.fillWidth: true
			Layout.preferredHeight: Layout.rowSpan
			Layout.preferredWidth: Layout.columnSpan
			Layout.columnSpan: 1
			Layout.rowSpan: 1
			Layout.column: 1
			Layout.row: 1
			enabled: false

			Material.accent: "white"

			BusyIndicator {
				running: loadButton.text == ""
				anchors.fill: parent
				anchors.centerIn: parent
			}

			Connections {
				target: QmlBridge
				onFuseReady: loadButton.state = "finished"
			}

			onClicked: {
				loadButton.state = "loading"
				QmlBridge.loadFuse()
			}

			states: [
				State {
					name: "loading"; 
					PropertyChanges { target: loadButton; text: "" }
					PropertyChanges { target: loadButton; disableBackgound: CSC.Style.primaryColor }
					PropertyChanges { target: loadButton; enabled: false }
					PropertyChanges { target: acceptFrame; enabled: false }
				},
				State {
					name: "finished"
					PropertyChanges { target: openButton; enabled: true }
					PropertyChanges { target: acceptFrame; enabled: false }
					PropertyChanges { target: loadButton; enabled: false }
					PropertyChanges { target: loadButton; text: "Refresh FUSE"}
				}
			]			
		}
	}
}