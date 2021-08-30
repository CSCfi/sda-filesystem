import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls.Material 2.12
import Qt.labs.qmlmodels 1.0
import QtQuick.Dialogs 1.3
import csc 1.0 as CSC

Page {
    id: page 

    property color bkgColor: CSC.Style.lightBlue
    property color lineColor: CSC.Style.tertiaryColor
    property FileDialog dialog

    header: ToolBar {
        padding: 4
        clip: true

        background: Rectangle {
            color: page.bkgColor

            Rectangle {
                height: 2
                color: page.lineColor
                anchors.bottom: parent.bottom
                anchors.left: parent.left
                anchors.right: parent.right
            }
        }

        contentItem: RowLayout {
            id: headerRow
            spacing: tableView.columnSpacing

            Item {
                id: levelItem
                Layout.preferredWidth: tableView.firstColumn

                Text {
                    id: levelTitle
                    text: "Level"
                    font.pointSize: 20
                    x: -tableView.contentX
                }

                /*Image {
                    source: "qrc:/qml/images/caret-down-fill.svg"
                    height: levelTitle.contentHeight / 2
                    fillMode: Image.PreserveAspectFit
                    anchors.left: levelTitle.right
                    anchors.leftMargin: 4
                    anchors.rightMargin: 4
                    anchors.verticalCenter: levelTitle.verticalCenter

                    RotationAnimator on rotation {
                        id: rotanim1
                        from: 0;
                        to: 180;
                        duration: 200
                        running: false
                    }

                    RotationAnimator on rotation {
                        id: rotanim2
                        from: 180;
                        to: 0;
                        duration: 200
                        running: false
                    }
                }*/
            }

            Item {
                Layout.preferredWidth: tableView.secondColumn

                Text {
                    text: "Date"
                    font.pointSize: 20
                    x: -tableView.contentX
                }
            }

            Item {
                Layout.preferredWidth: messageText.contentWidth

                Text {
                    id: messageText
                    text: "Message"
                    font.pointSize: 20
                    x: -tableView.contentX
                }
            }

            Item {
                implicitHeight: exportButton.implicitHeight
                Layout.minimumWidth: headerRow.spacing + exportButton.implicitWidth
                Layout.fillWidth: true
                Layout.margins: 4

                ToolButton {
                    id: exportButton
                    text: "Export"
                    icon.source: "qrc:/qml/images/box-arrow-up.svg"
                    anchors.right: parent.right
                    anchors.rightMargin: (header.availableWidth >= headerRow.implicitWidth || tableView.contentX <= 0) ? 0 : 
                        Math.min(tableView.contentX, headerRow.implicitWidth - header.availableWidth)

                    onClicked: dialog.visible = true

                    background: Rectangle {
                        border.width: 2
                        border.color: "black"
                        color: exportButton.hovered ? CSC.Style.lightGrey : "transparent"
                        radius: 5
                    }

                    MouseArea {
                        cursorShape: Qt.PointingHandCursor
                        acceptedButtons: Qt.NoButton
                        anchors.fill: parent
                    }
                }
            }
        }
    }

    /*Popup {
        id: levelMenu
        topPadding: 0
        bottomPadding: 0
        margins: 0

        onAboutToShow: rotanim1.start()
        onAboutToHide: rotanim2.start()

        contentItem: ColumnLayout {
            spacing: 0
            Material.accent: CSC.Style.primaryColor

            CheckBox {
                text: "Error"
                checked: true
            }
            CheckBox {
                text: "Warning"
                checked: true
            }
            CheckBox {
                text: "Info"
                checked: true
            }
        }

        background: Rectangle {
            radius: 0
        }
    }*/

    TableView {
        id: tableView
        anchors.fill: parent
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        columnSpacing: 20

        property bool ready: false
        property real firstColumn: 1
        property real secondColumn: 1
        property real thirdColumn: 1
        property real rowHeight: 40

        model: LogModel
        delegate: chooser

        ScrollBar.vertical: ScrollBar { }
        ScrollBar.horizontal: ScrollBar { }

        Component.onCompleted: LogModel.removeDummy()
        onThirdColumnChanged: timer.restart()

        rowHeightProvider: function (column) { return rowHeight }
        columnWidthProvider: function (column) { return column == 0 ? firstColumn : column == 1 ? secondColumn : thirdColumn }

        Image {
            source: "qrc:/qml/images/bkg-log-rect.png"
            fillMode: Image.TileVertically
            verticalAlignment: Image.AlignTop
            width: Math.max(tableView.width, tableView.contentWidth)
            height: Math.max(tableView.height, tableView.contentHeight)
            smooth: false
        }

        // Timer is needed so that forceLayout() is called after event loop and QML doesn't complain
        Timer {
            id: timer
            interval: 0; running: false; repeat: false
            onTriggered: tableView.forceLayout()
        }
    }

    DelegateChooser {
        id: chooser

        DelegateChoice {
            column: 0
            delegate: Control {
                padding: 10

                onWidthChanged: {
                    if (width > tableView.firstColumn) {
                        tableView.firstColumn = width
                    }
                }

                contentItem: Label {
                    id: levelText
                    text: {
                        switch (level) {
                            case LogLevel.Error:
                                return "ERROR"
                            case LogLevel.Info:
                                return "INFO"
                            case LogLevel.Debug:
                                return "DEBUG"
                            case LogLevel.Warning:
                                return "WARNING"
                            default:
                                return ""
                        }
                    }
                    color: {
                        switch (level) {
                            case LogLevel.Error:
                            case LogLevel.Info:
                            case LogLevel.Debug:
                                return "white"
                            case LogLevel.Warning:
                                return "black"
                            default:
                                return "transparent"
                        }
                    }
                    topPadding: 0
                    bottomPadding: 0
                    leftPadding: 5
                    rightPadding: 5
                    verticalAlignment: Text.AlignVCenter
                    horizontalAlignment: Text.AlignHCenter
                    font.capitalization: Font.AllUppercase
                    anchors.centerIn: parent

                    background: Rectangle {
                        color: {
                            if (level == LogLevel.Info) {
                                return CSC.Style.blue
                            } else if (level == LogLevel.Error) {
                                return CSC.Style.red
                            } else if (level == LogLevel.Warning) {
                                return CSC.Style.yellow
                            } else if (level == LogLevel.Debug) {
                                return CSC.Style.altGreen
                            } else {
                                return "transparent"
                            }
                        }
                        radius: height / 2
                    }
                }
            }
        }

        DelegateChoice {
            column: 1
            delegate: Label { 
                text: timestamp
                padding: 5
                verticalAlignment: Text.AlignVCenter
                color: "black"

                onContentWidthChanged: {
                    if (contentWidth + 2 * padding > tableView.secondColumn) {
                        tableView.secondColumn = contentWidth + 2 * padding
                    }
                }
            }
        }

        DelegateChoice {
            column: 2
            delegate: Label { 
                id: messageLabel
                text: message[0]
                verticalAlignment: Text.AlignVCenter
                padding: 5
                color: "black"

                onContentWidthChanged: {
                    if (contentWidth + 2 * padding > tableView.thirdColumn) {
                        tableView.thirdColumn = contentWidth + 2 * padding 
                    }
                    /*if (text == "") {
                        messageLabel.grabToImage(function(result) {
                            result.saveToFile("bkg-log-rect.png");
                        });
                    }*/
                }

                //background: Loader { sourceComponent: bkg }
            }
        }
    }

    // THIS IS IMPORTANT
    // Uncommenting the comments in messageLabel creates bkg-log-rect.png which can then be used 
    // as background for logs after recompiling. Remember to recomment and move the new .png to /images
    // I do it like this because this seamlessly (hopefully) fills in the background
    // regardless of row widths and row counts
    Component {
        id: bkg

        Rectangle {
            color: page.bkgColor

            Rectangle {
                color: page.lineColor
                height: 1
                anchors.top: parent.top
                anchors.right: parent.right
                anchors.left: parent.left
            }

            Rectangle {
                color: page.lineColor
                height: 1
                anchors.bottom: parent.bottom
                anchors.right: parent.right
                anchors.left: parent.left
            }
        }
    }
}