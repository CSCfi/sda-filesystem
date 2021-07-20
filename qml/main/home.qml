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
	color: CSC.Style.secondaryColor

	property string username

	/*header: TabBar {
		Material.accent: CSC.Style.secondaryColor

        TabButton {
        	text: qsTr("Home")
			width: implicitWidth
    	}
		TabButton {
        	text: qsTr("Logs")
			width: implicitWidth
    	}
		TabButton {
        	text: qsTr("Statistics?")
			width: implicitWidth
    	}
    }*/

	header: ToolBar {
        RowLayout {
            anchors.fill: parent
            ToolButton {
                text: "‹"
                onClicked: stack.pop()
            }
            /*Label {
                text: "Title"
                elide: Label.ElideRight
                horizontalAlignment: Qt.AlignHCenter
                verticalAlignment: Qt.AlignVCenter
                Layout.fillWidth: true
            }*/
            ToolButton {
                text: username + " \u142F"
                onClicked: menu.open()
            }
        }
    }

	ColumnLayout {
		anchors.fill: parent

		Label {
			text: "<h2>Logged in as " + username + "</h2>"
			color: "black"
		}

		GridLayout {
			columns: 2
			Layout.fillHeight: true
			Layout.fillWidth: true
			//Layout.alignment: Qt.AlignTop

			Frame {
				Layout.fillWidth: true

				background: Rectangle {
					color: CSC.Style.lightGreen
				}

				ColumnLayout {
					anchors.fill: parent

					Text {
						text: "<h3>FUSE will be mounted at:</h3>"
					}

					CSC.TextField {
						text: qmlBridge.mountPoint
						Layout.fillWidth: true
					}

					Button {
						text: "Accept"
						Layout.alignment: Qt.AlignRight
					}
				}
			}

			Frame {
				//Layout.fillHeight: true
				Layout.fillWidth: true

				ColumnLayout {
					anchors.fill: parent

					Text {
						text: "Your FUSE contains:<br/><ul><li>0 Project(s)</li><li>0 Container(s)</li><li>0 Object(s)</li></ul>"
					}

					RowLayout {
						Button {
							text: "Open FUSE"
						}

						Button {
							text: "Load FUSE"
							onClicked: qmlBridge.loadFuse()
						}
					}
				}	
			}
		}

		GroupBox { // groupbox?
			title: "Projects"
			Layout.fillHeight: true
			Layout.fillWidth: true

			ListView { // go struct

			}
		}
	}
	
}