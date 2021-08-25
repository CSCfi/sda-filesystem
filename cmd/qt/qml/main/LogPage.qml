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

    header: ToolBar {
        Material.primary: page.bkgColor

        Rectangle {
            height: 2
            color: page.lineColor
            anchors.bottom: parent.bottom
            anchors.left: parent.left
            anchors.right: parent.right
        }

        Row {
            anchors.fill: parent
            spacing: tableView.columnSpacing
            leftPadding: 3
            bottomPadding: 3

            Text {
                id: levelTitle
                text: "Level"
                height: parent.height
                width: tableView.firstColumn
                font.pointSize: 25
                verticalAlignment: Text.AlignBottom
            }

            Text {
                text: "Date"
                height: parent.height
                width: tableView.secondColumn
                font.pointSize: 25
                verticalAlignment: Text.AlignBottom
            }

            Text {
                text: "Message"
                height: parent.height
                width: tableView.thirdColumn
                font.pointSize: 25
                verticalAlignment: Text.AlignBottom
            }
        }

        ToolButton {
            id: exportButton
            text: "Export"
            icon.source: "qrc:/qml/images/box-arrow-up.svg"
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            anchors.rightMargin: 5

            onClicked: fileDialog.visible = true

            background: Rectangle {
                border.width: 2
                border.color: "black"
                color: exportButton.hovered ? CSC.Style.lightGrey : "transparent"
                radius: 5
            }
        }
    }

    FileDialog {
        id: fileDialog
        title: "Choose file to which save logs"
        folder: shortcuts.home
        selectExisting: false
        selectFolder: false
        defaultSuffix: "log"
        onAccepted: LogModel.saveLogs(fileDialog.fileUrl)
    }

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
                    text: level
                    color: {
                        if (levelText.text == "INFO" || levelText.text == "ERROR" || levelText.text == "DEBUG") {
                            return "white"
                        } else if (levelText.text == "WARNING") {
                            return "black"
                        } else {
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
                            if (levelText.text == "INFO") {
                                return CSC.Style.blue
                            } else if (levelText.text == "ERROR") {
                                return CSC.Style.red
                            } else if (levelText.text == "WARNING") {
                                return CSC.Style.yellow
                            } else if (levelText.text == "DEBUG") {
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
                text: message.split('\n')[0]
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
    // Uncommenting the comments in messageLabel creates bkg-log-rect.pn which can then be used 
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