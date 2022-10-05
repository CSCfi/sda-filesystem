import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls.Material 2.12
import csc 1.2 as CSC

Page {
    id: page 
    height: table.height + implicitHeaderHeight + 3 * CSC.Style.padding
    implicitWidth: table.implicitWidth + rightPadding + leftPadding
    topPadding: CSC.Style.padding
    bottomPadding: 2 * CSC.Style.padding
    rightPadding: 2 * CSC.Style.padding
    leftPadding: 2 * CSC.Style.padding

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
                font.weight: Font.DemiBold
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
        font.pixelSize: 13
        font.weight: Font.DemiBold
    }

    TextMetrics {
        id: textMetricsDate
        text: "0000-00-00 00:00:00"
        font: table.contentFont
    }

    CSC.Table {
        id: table
        width: parent.width
        modelSource: LogModel.proxy
        contentSource: logLine
        footerSource: footerLine
        objectName: "logs"
        focus: true
    }

    Menu {
        id: menu
        y: table.footerItem.height
        width: 0

        Material.accent: CSC.Style.primaryColor

        Repeater {
            model: LogModel.includeDebug ? levels.concat(LogLevel.Debug) : levels
        
            property var levels: [LogLevel.Error, LogLevel.Warning, LogLevel.Info] 

            MenuItem { 
                id: menuItem
                text: LogModel.getLevelStr(modelData)
                topPadding: 7
                bottomPadding: 7

                Material.foreground: CSC.Style.grey

                contentItem: CheckBox {
                    text: menuItem.text
                    checked: true
                    padding: 0
                    font.weight: Font.DemiBold

                    onCheckedChanged: LogModel.toggleFilteredLevel(modelData, checked)
                    Component.onCompleted: menu.width = Math.max(menu.width, implicitContentWidth + implicitIndicatorWidth + 2 * menuItem.padding)
                }
            }
        }

        background: Rectangle {
            implicitWidth: menu.width
            color: "white"
            border.width: 1
            border.color: CSC.Style.lightGrey
        }
    }

    Component {
        id: footerLine

        RowLayout {
            RowLayout {
                spacing: 0
                Layout.preferredWidth: textMetricsLevel.width + 30
                Layout.fillHeight: true

                Label {
                    text: "Level"
                }

                RoundButton {
                    id: filterLevel
                    padding: 10
                    flat: true
                    checkable: true
                    icon.source: "qrc:/qml/images/filter.svg"
                    icon.color: checked ? CSC.Style.primaryColor : CSC.Style.grey
                    icon.height: 25
                    icon.width: 25

                    onCheckedChanged: menu.visible = checked

                    background: Rectangle {
                        radius: filterLevel.height * 0.5
                        color: filterLevel.checked ? CSC.Style.lightBlue : "transparent"
                    }

                    Connections {
                        target: menu
                        onClosed: filterLevel.checked = false
                    }

                    MouseArea {
                        cursorShape: Qt.PointingHandCursor
                        acceptedButtons: Qt.NoButton
                        anchors.fill: parent
                    }
                }
            }

            Label {
                text: "Date and Time"
                Layout.preferredWidth: textMetricsDate.width
            }

            Label {
                text: "Message"
                Layout.fillWidth: true
            }
        }
    }

    Component {
        id: logLine

        RowLayout {
            height: messageLabel.height

            property int level: modelData ? modelData.level : -1
            property string timestamp: modelData ? modelData.timestamp : ""
            property var message: modelData ? modelData.message : null

            Label {
                id: levelText
                text: LogModel.getLevelStr(level)
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
                font: textMetricsLevel.font
                Layout.preferredWidth: textMetricsLevel.width + 30

                background: Rectangle {
                    color: {
                        switch (level) {
                            case LogLevel.Info:
                                return "#EEF2F7"
                            case LogLevel.Error:
                                return "#F5E6E9"
                            case LogLevel.Warning:
                                return "#FEF7E5"
                            case LogLevel.Debug:
                                return "#E7F1DC"
                            default:
                                return "transparent"
                        }
                    }
                    border.color: levelText.color
                    border.width: 1
                    radius: height / 6
                }
            }

            Label {
                text: timestamp
                Layout.preferredWidth: textMetricsDate.width
            }

            Label {
                id: messageLabel
                text: message ? message[0] : ""
                wrapMode: Text.Wrap
                topPadding: 10
                bottomPadding: 10
                lineHeight: 1.2
                verticalAlignment: Text.AlignVCenter
                Layout.fillWidth: true
            }
        }
    }
}
