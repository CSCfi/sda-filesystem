import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick.Dialogs 1.3
import QtQuick.Controls.Material 2.12
import QtQuick.Window 2.13
import QtQml 2.13
import csc 1.0 as CSC

ApplicationWindow {
    id: window
    title: "SDA Filesystem"
    visible: true
    visibility: "Maximized"
    minimumWidth: Math.max(header.implicitWidth, login.implicitWidth, logs.implicitWidth)
    minimumHeight: header.implicitHeight + login.implicitHeight
    font.capitalization: Font.MixedCase
    
    Material.background: "white"
    
    // Ensures fuse unmounts when application terminates
	onClosing: QmlBridge.shutdown()

    header: ToolBar {
        leftPadding: CSC.Style.padding
        rightPadding: CSC.Style.padding

        Material.primary: "white"

        contentItem: RowLayout {
            spacing: CSC.Style.padding

            RowLayout {
                Layout.topMargin: 0.5 * CSC.Style.padding
                Layout.bottomMargin: 0.5 * CSC.Style.padding

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

            Rectangle {
                Layout.fillWidth: true
            }

            TabBar {
				id: tabBar
                spacing: CSC.Style.padding
                contentHeight: height
                Layout.fillHeight: true

                Material.accent: CSC.Style.primaryColor

                background: Rectangle {
                    color: "white"
                }

                Repeater {
                    model: ["Home", "Logs"]

                    TabButton {
                        id: tabButton
                        text: modelData
                        width: implicitWidth
                        height: tabBar.height

                        contentItem: Text {
                            text: tabButton.text
                            font: tabButton.font
                            color: CSC.Style.primaryColor
                            horizontalAlignment: Text.AlignHCenter
                            verticalAlignment: Text.AlignVCenter
                            maximumLineCount: 1
                        }
                    }
                }
			}

            Rectangle {
                Layout.fillWidth: true
            }

            ToolButton {
                id: signout
                text: "Sign out"
                enabled: stack.state == "loggedIn"
                opacity: enabled ? 1 : 0
                icon.source: "qrc:/qml/images/box-arrow-right.svg"
                LayoutMirroring.enabled: true
                Layout.fillHeight: true

                Material.foreground: CSC.Style.primaryColor

                MouseArea {
                    cursorShape: Qt.PointingHandCursor
                    acceptedButtons: Qt.NoButton
                    anchors.fill: parent
                }

                onClicked: close()
            }
        }
    }

    FileDialog {
        id: dialogSave
        title: "Choose file to which save logs"
        folder: shortcuts.home
        selectExisting: false
        selectFolder: false
        defaultSuffix: "log"

		signal ready

        onAccepted: { LogModel.saveLogs(dialogSave.fileUrl); ready() }
    }

    CSC.Popup {
		id: popup
	}

    Connections {
		target: QmlBridge
		onInitError: {
			login.enabled = false
			popup.errorMessage = message + ". Check logs for further details and rerun the application"
            popup.closePolicy = Popup.NoAutoClose
            popup.modal = false
			popup.open()
		}
	}

    StackLayout {
        id: stack
        currentIndex: tabBar.currentIndex
        anchors.fill: parent

        Flickable {
            interactive: contentHeight > height
            contentHeight: login.height

            ScrollBar.vertical: ScrollBar { }

            LoginPage {
                id: login
                anchors.horizontalCenter: parent.horizontalCenter

                onLoggedInChanged: {
                    if (loggedIn) {
                        stack.state = "loggedIn"
                    }
                }
            }
        }
        
        Flickable {
            interactive: contentHeight > height
            contentHeight: logs.height

            ScrollBar.vertical: ScrollBar { }

            LogPage {
                id: logs
                width: parent.width
            }
        }

        Flickable {
            interactive: contentHeight > height
            contentHeight: front.height

            ScrollBar.vertical: ScrollBar { }

            FrontPage {
                id: front
                width: parent.width
            }
        }

        states: [
            State {
                name: "loggedIn"
                PropertyChanges {
                    target: stack
                    currentIndex: 2 - tabBar.currentIndex
                }
            }
        ]
    }
}