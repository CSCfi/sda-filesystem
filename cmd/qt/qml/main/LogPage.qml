import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQml.Models 2.13
import QtQuick.Controls.Material 2.12
import Qt.labs.qmlmodels 1.0
import QtQuick.Dialogs 1.3
import csc 1.0 as CSC

Page {
    id: page 
    height: table.height + implicitHeaderHeight + 4 * CSC.Style.padding
    implicitWidth: table.implicitWidth + 4 * CSC.Style.padding
    padding: 2 * CSC.Style.padding

    header: Control {
        topPadding: 2 * CSC.Style.padding
        rightPadding: 2 * CSC.Style.padding
        leftPadding: 2 * CSC.Style.padding

        contentItem: RowLayout { 
            Label {
                text: "<h1>Logs</h1>"
                color: CSC.Style.grey
                verticalAlignment: Text.AlignVCenter
                maximumLineCount: 1
                Layout.fillWidth: true
                Layout.fillHeight: true
            }

            ToolButton {
                id: exportButton
                text: "Export detailed logs"
                icon.source: "qrc:/qml/images/download.svg"
                Layout.alignment: Qt.AlignRight

                Material.foreground: CSC.Style.primaryColor

                onClicked: dialogSave.visible = true

                MouseArea {
                    cursorShape: Qt.PointingHandCursor
                    acceptedButtons: Qt.NoButton
                    anchors.fill: parent
                }
            }
        }
    }

    TextMetrics {
        id: textMetricsLevel
        text: "Warning"
        font.pointSize: 13
        font.weight: Font.Medium
    }

    TextMetrics {
        id: textMetricsDate
        text: "0000-00-00 00:00:00"
        font.pointSize: 15
    }

    CSC.Table {
        id: table
        width: parent.width
        modelSource: LogModel
        delegateSource: logLine
        objectName: "logs"

        footer: Rectangle {
            height: 50
            width: table.width
            border.width: 1
            border.color: CSC.Style.lightGrey

            RowLayout {
                spacing: 30
                anchors.fill: parent
                anchors.leftMargin: CSC.Style.padding
                anchors.rightMargin: CSC.Style.padding

                Text {
                    id: levelText
                    text: "Level"
                    font.pointSize: 13
                    font.weight: Font.Medium
                    Layout.preferredWidth: textMetricsLevel.width + 30
                }

                Text {
                    text: "Date and Time"
                    font.pointSize: 13
                    font.weight: Font.Medium
                    Layout.preferredWidth: textMetricsDate.width
                }

                Text {
                    id: messageLabel
                    text: "Message"
                    font.pointSize: 13
                    font.weight: Font.Medium
                    Layout.fillWidth: true
                }
            }
        }
    }

    Component {
        id: logLine

        Rectangle {
            height: childrenRect.height
            width: table.width
            border.width: 1
            border.color: CSC.Style.lightGrey

            RowLayout {
                spacing: 30
                height: Math.max(60, messageLabel.height)
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.leftMargin: CSC.Style.padding
                anchors.rightMargin: CSC.Style.padding

                Label {
                    id: levelText
                    text: {
                        switch (level) {
                            case LogLevel.Error:
                                return "Error"
                            case LogLevel.Info:
                                return "Info"
                            case LogLevel.Debug:
                                return "Debug"
                            case LogLevel.Warning:
                                return "Warning"
                            default:
                                return ""
                        }
                    }
                    color: {
                        switch (level) {
                            case LogLevel.Error:
                                return "#A9252F"
                            case LogLevel.Info:
                                return "#102E5C"
                            case LogLevel.Debug:
                                return "#4B7923"
                            case LogLevel.Warning:
                                return "#B84F20"
                            default:
                                return "transparent"
                        }
                    }
                    topPadding: 5
                    bottomPadding: 5
                    horizontalAlignment: Text.AlignHCenter
                    font.pointSize: 13
                    font.weight: Font.Medium
                    Layout.preferredWidth: textMetricsLevel.width + 30
                    Layout.alignment: Qt.AlignVCenter

                    background: Rectangle {
                        color: {
                            if (level == LogLevel.Info) {
                                return "#EEF2F7"
                            } else if (level == LogLevel.Error) {
                                return "#F5E6E9"
                            } else if (level == LogLevel.Warning) {
                                return "#FEF7E5"
                            } else if (level == LogLevel.Debug) {
                                return "#E7F1DC"
                            } else {
                                return "transparent"
                            }
                        }
                        border.color: levelText.color
                        border.width: 1
                        radius: height / 6
                    }
                }

                Text {
                    text: timestamp
                    font.pointSize: 15
                    Layout.preferredWidth: textMetricsDate.width
                    Layout.alignment: Qt.AlignVCenter
                }

                Text {
                    id: messageLabel
                    text: message[0]
                    wrapMode: Text.Wrap
                    font.pointSize: 15
                    topPadding: 10
                    bottomPadding: 10
                    lineHeight: 1.2
                    Layout.fillWidth: true
                    Layout.alignment: Qt.AlignVCenter
                }
            }
        }
    }
}
