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

        RowLayout {
            anchors.fill: parent
            spacing: tableView.columnSpacing
            anchors.margins: 3

            Text {
                id: levelTitle
                text: "Level"
                Layout.fillHeight: parent.height
                Layout.preferredWidth: tableView.firstColumn
                font.pointSize: 25
                verticalAlignment: Text.AlignBottom
            }

            Text {
                text: "Date"
                Layout.fillHeight: parent.height
                Layout.preferredWidth: tableView.secondColumn
                font.pointSize: 25
                verticalAlignment: Text.AlignBottom
            }

            Text {
                text: "Message"
                Layout.fillHeight: parent.height
                Layout.preferredWidth: tableView.thirdColumn
                font.pointSize: 25
                verticalAlignment: Text.AlignBottom
            }

            Rectangle {
                Layout.fillWidth: true
            }

            ToolButton {
                id: exportButton
                text: "Export"
                icon.source: "qrc:/qml/images/box-arrow-up.svg"

                onClicked: fileDialog.visible = true

                background: Rectangle {
                    border.width: 2
                    border.color: "black"
                    color: exportButton.hovered ? CSC.Style.lightGrey : "transparent"
                    radius: 5
                }
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
        //clip: true
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
                    color: (levelText.text.toLowerCase() != "warning") ? "white" : "black"
                    topPadding: 0
                    bottomPadding: 0
                    leftPadding: 5
                    rightPadding: 5
                    verticalAlignment: Text.AlignVCenter
                    horizontalAlignment: Text.AlignHCenter
                    font.capitalization: Font.AllUppercase
                    anchors.centerIn: parent

                    background: Rectangle {
                        color: (levelText.text.toLowerCase() == "info") ? CSC.Style.blue : 
                               (levelText.text.toLowerCase() == "error" ? CSC.Style.red : CSC.Style.yellow)
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
    // Uncommenting the comments in DelegateChoice for column 2 creates bkg-log-rect.png
    // which can then be used as background for logs. Remember to recomment and move the new .png to /images
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