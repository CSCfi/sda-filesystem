import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick.Dialogs 1.3
import QtQuick.Controls.Material 2.12
import QtQuick.Window 2.13
import QtQml 2.13
import csc 1.2 as CSC

ApplicationWindow {
    id: window
    title: "Data Gateway"
    visible: true
    minimumWidth: Math.max(header.implicitWidth, logs.implicitWidth)
    minimumHeight: header.implicitHeight + login.implicitHeight
    width: minimumWidth
    height: minimumHeight
    flags: Qt.Window | Qt.CustomizeWindowHint | Qt.WindowTitleHint | Qt.WindowSystemMenuHint | Qt.WindowMinMaxButtonsHint | Qt.WindowFullscreenButtonHint | Qt.WindowCloseButtonHint
    font.capitalization: Font.MixedCase

    Material.background: "white"
    
    //onActiveFocusItemChanged: print("activeFocusItem", activeFocusItem)

    // Ensures fuse unmounts when application terminates
    onClosing: QmlBridge.shutdown()

    Component.onCompleted: {
        x = Screen.virtualX + 0.5 * (Screen.desktopAvailableWidth - width)
        y = Screen.virtualY + 0.5 * (Screen.desktopAvailableHeight - height)
    }

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

                Label {
                    text: "<h4>Data Gateway</h4>"
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

                property bool accessed: false

                Material.accent: CSC.Style.primaryColor

                background: Rectangle {
                    color: "white"
                }

                Repeater {
                    id: repeater
                    model: ["Login", "Logs"]

                    TabButton {
                        id: tabButton
                        text: modelData
                        width: implicitWidth
                        height: tabBar.height
                        enabled: !QmlBridge.loggedIn || tabBar.accessed || text == "Access" || text == "Logs"
                        font.weight: Font.DemiBold

                        Material.foreground: enabled ? CSC.Style.primaryColor : CSC.Style.lightGrey

                        contentItem: Label {
                            text: tabButton.text
                            font: tabButton.font
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
                text: "Disconnect and sign out"
                enabled: stack.state == "loggedIn"
                opacity: enabled ? 1 : 0
                font.weight: Font.DemiBold
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

        onAccepted: { 
            LogModel.saveLogs(dialogSave.fileUrl)

            if (quitButton.checked) {
                close()
            }
        }
    }

    CSC.Popup {
        id: popup
    }

    CSC.Popup {
        id: popupPanic
        errorMessage: "How can this be! Data Gateway failed to load correctly.\nSave logs to find out why this happened and either quit the application or continue at your own peril..."
        
        Row {
            spacing: CSC.Style.padding
            anchors.right: parent.right

            CSC.Button {
                text: "Ignore"
                outlined: true

                onClicked: popupPanic.close()
            }

            CSC.Button {
                id: quitButton
                text: "Save logs and quit"
                checkable: true
                
                onClicked: dialogSave.visible = true
            }
        }
    }

    Connections {
        target: QmlBridge
        onInitError: {
            login.enabled = false
            popup.errorMessage = message + ". Check logs for further details and rerun the application."
            popup.closePolicy = Popup.NoAutoClose
            popup.modal = false
            popup.open()
        }
        onPopupError: {
            popup.errorMessage = message
            popup.open()
        }
        onPanic: {
            popupPanic.closePolicy = Popup.NoAutoClose // User must choose ignore or quit
            popupPanic.open()
        }
        onLoggedInChanged: if (QmlBridge.loggedIn) {
            repeater.model = ["Access", "Export", "Logs"]
            stack.state = "loggedIn"
            window.flags = window.flags & ~Qt.WindowCloseButtonHint
        }
        onFuseReady: tabBar.accessed = true
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
                focus: visible
                anchors.horizontalCenter: parent.horizontalCenter
            }
        }
        
        Flickable {
            interactive: contentHeight > height
            contentHeight: logs.height

            ScrollBar.vertical: ScrollBar { }

            LogsPage {
                id: logs
                focus: visible
                width: parent.width
            }
        }

        Flickable {
            interactive: contentHeight > height
            contentHeight: exp.height

            ScrollBar.vertical: ScrollBar { }

            ExportPage {
                id: exp
                focus: visible
                width: parent.width
            }
        }

        Flickable {
            interactive: contentHeight > height
            contentHeight: access.height

            ScrollBar.vertical: ScrollBar { }

            AccessPage {
                id: access
                focus: visible
                width: parent.width
            }
        }

        states: [
            State {
                name: "loggedIn"
                PropertyChanges {
                    target: stack
                    currentIndex: 3 - tabBar.currentIndex
                }
            }
        ]
    }
}