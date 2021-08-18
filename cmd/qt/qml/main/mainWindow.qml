import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls.Material 2.12
import QtQuick.Window 2.13
import csc 1.0 as CSC

ApplicationWindow {
	id: mainWindow
    visible: true
    title: "SD-Connect Filesystem"
	width: 1000
	height: 600
	minimumHeight: sideBarView.height + toolbar.height
	minimumWidth: homePage.implicitWidth + sideBarView.width

	property string username

	Material.primary: CSC.Style.primaryColor

	header: ToolBar {
		id: toolbar

        RowLayout {
            anchors.fill: parent

            Image {
				source: "qrc:/qml/images/CSC_logo.svg"
				fillMode: Image.PreserveAspectFit
				Layout.preferredWidth: paintedWidth
				Layout.preferredHeight: 50
				Layout.margins: 5
			}

            Text {
                text: username
				color: "white"
				rightPadding: 10
				font.pointSize: 15
				font.weight: Font.DemiBold
				Layout.alignment: Qt.AlignRight
            }
        }
    }

	onClosing: QmlBridge.shutdown()

	RowLayout {
		id: body
		spacing: 0
		anchors.fill: parent

		Rectangle {
			color: CSC.Style.tertiaryColor
			Material.foreground: "white"
			Layout.fillHeight: true
			Layout.preferredWidth: 200

			Component {
				id: separator
				Rectangle {
					height: section != "main" ? 30 : 0
				}
			}

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
						section: "main"
					}
					ListElement {
						name: "Logs"
						image: "qrc:/qml/images/layout-text-sidebar-reverse.svg"
						section: "main"
					}
					ListElement {
						name: "Logout"
						image: "qrc:/qml/images/box-arrow-right.svg"
						section: "end"
					}
				}

				highlight: Rectangle { 
					color: CSC.Style.secondaryColor
				}

				delegate: ItemDelegate {
					text: name
					icon.source: image
					width: sideBarView.width
					highlighted: sideBarView.highlightedIndex == index

					onClicked: {
						if (section == "end") {
							close()
						} else {
							if (sideBarView.currentIndex != index) {
								sideBarView.currentIndex = index
							}
						}
					}
				}

				section.property: "section"
				section.criteria: ViewSection.FullString
				section.delegate: separator
			}
		}

		StackLayout {
			id: stack
			currentIndex: sideBarView.currentIndex
			Layout.fillWidth: true
			Layout.fillHeight: true

			Flickable {
				interactive: contentHeight > height
				contentHeight: homePage.implicitHeight
				implicitWidth: homePage.implicitWidth

				ScrollBar.vertical: ScrollBar { }

				HomePage {
					id: homePage
					topItem: body
					height: stack.height
					width: parent.width
				}
			}

			LogPage {
				id: logPage
			}
		}
	}
}
