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
    height: listView.height + implicitHeaderHeight + 4 * CSC.Style.padding
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
                text: "Export logs"
                icon.source: "qrc:/qml/images/box-arrow-up.svg"
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

    ListView {
        id: listView
        width: parent.width
        height: contentHeight
        model: visualModel
        interactive: false
        boundsBehavior: Flickable.StopAtBounds
        verticalLayoutDirection: ListView.BottomToTop

        property int amountVisible: 5
        property int page: 1
        property int maxPages: Math.ceil(LogModel.count / listView.amountVisible)

        onPageChanged: {
            visibleItems.setGroups(0, visibleItems.count, "items")
        }

        footer: Rectangle {
            height: 50
            width: listView.width
            border.width: 1
            border.color: CSC.Style.lightGrey

            RowLayout {
                spacing: 30
                anchors.fill: parent
                anchors.leftMargin: CSC.Style.padding

                Label {
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

        header: Rectangle {
            height: 40
            width: listView.width
            implicitWidth: pagination.implicitWidth
            border.width: 1
            border.color: CSC.Style.lightGrey

            onWidthChanged: console.log(width, implicitWidth)

            Text {
                text: "No logs available"
                visible: LogModel.count == 0
                verticalAlignment: Text.AlignVCenter
                font.pointSize: 15
                anchors.fill: parent
                anchors.leftMargin: CSC.Style.padding
            }

            RowLayout {
                id: pagination
                spacing: 10
                visible: LogModel.count > 0
                anchors.fill: parent
                anchors.leftMargin: CSC.Style.padding

                Material.foreground: CSC.Style.primaryColor

                RowLayout {
                    spacing: 10
                    Layout.fillHeight: true

                    Text {
                        text: "Items per page: "
                        Layout.preferredWidth: contentWidth
                    }

                    ToolButton {
                        text: listView.amountVisible + "  "
                        font.pointSize: 15
                        icon.source: "qrc:/qml/images/chevron-down.svg"
                        LayoutMirroring.enabled: true
                        Layout.fillHeight: true
                        Layout.preferredWidth: 1.5 * implicitWidth

                        background: Rectangle {
                            border.width: 1
                            border.color: CSC.Style.lightGrey
                            color: parent.hovered ? CSC.Style.lightGrey : "white"
                        }

                        MouseArea {
                            cursorShape: Qt.PointingHandCursor
                            acceptedButtons: Qt.NoButton
                            anchors.fill: parent
                        }

                        onClicked: menu.open()

                        Menu {
                            id: menu

                            Repeater {
                                model: 4
                                MenuItem {
                                    text: amount //Array.from(Array(4), (_,i)=> 5 + 5 * i)

                                    property int amount
                                    
                                    Component.onCompleted: amount = (index + 1) * listView.amountVisible
                                    onTriggered: listView.amountVisible = amount
                                }
                            }
                        }
                    }
                }

                Rectangle {
                    Layout.fillWidth: true
                }

                Text {
                    text: firstIdx + " - " + lastIdx + " of " + LogModel.count + " items"
                    wrapMode: Text.WordWrap

                    property int firstIdx: (listView.page - 1) * listView.amountVisible + 1
                    property int lastIdx: {
                        if (LogModel.count < listView.amountVisible) {
                            return LogModel.count
                        } else {
                            return firstIdx + listView.amountVisible - 1
                        }
                    }
                }

                Rectangle {
                    Layout.fillWidth: true
                }

                Text {
                    text: listView.page + " of " + listView.maxPages + " pages"
                    wrapMode: Text.WordWrap
                }

                Rectangle {
                    Layout.fillWidth: true
                }

                RowLayout {
                    Layout.fillHeight: true

                    ToolButton {
                        icon.source: "qrc:/qml/images/chevron-left.svg"
                        Layout.fillHeight: true
                        Layout.preferredWidth: height

                        onClicked: listView.page =  Math.max(1, listView.page - 1)

                        background: Rectangle {
                            border.width: 1
                            border.color: CSC.Style.lightGrey
                            color: parent.hovered ? CSC.Style.lightGrey : "white"
                        }

                        MouseArea {
                            cursorShape: Qt.PointingHandCursor
                            acceptedButtons: Qt.NoButton
                            anchors.fill: parent
                        }
                    }

                    Repeater {
                        model: 5

                        ToolButton {
                            text: index + 1
                            Layout.fillHeight: true
                            Layout.preferredWidth: height

                            Material.foreground: (index + 1) != listView.page ? CSC.Style.grey : CSC.Style.primaryColor

                            MouseArea {
                                cursorShape: Qt.PointingHandCursor
                                acceptedButtons: Qt.NoButton
                                anchors.fill: parent
                            }
                        }
                    }

                    ToolButton {
                        enabled: false
                        Layout.fillHeight: true
                        Layout.preferredWidth: height
                        icon.source: "qrc:/qml/images/three-dots.svg"
                    }

                    ToolButton {
                        text: listView.maxPages
                        Layout.fillHeight: true
                        Layout.preferredWidth: height

                        Material.foreground: listView.maxPages != listView.page ? CSC.Style.grey : CSC.Style.primaryColor

                        MouseArea {
                            cursorShape: Qt.PointingHandCursor
                            acceptedButtons: Qt.NoButton
                            anchors.fill: parent
                        }
                    }

                    ToolButton {
                        icon.source: "qrc:/qml/images/chevron-right.svg"
                        Layout.fillHeight: true
                        Layout.preferredWidth: implicitWidth

                        onClicked: listView.page =  Math.min(listView.maxPages, listView.page + 1)

                        background: Rectangle {
                            border.width: 1
                            border.color: CSC.Style.lightGrey
                            color: parent.hovered ? CSC.Style.lightGrey : "white"
                        }

                        MouseArea {
                            cursorShape: Qt.PointingHandCursor
                            acceptedButtons: Qt.NoButton
                            anchors.fill: parent
                        }
                    }
                }

                onWidthChanged: {
                    console.log(width, implicitWidth, childrenRect.width, parent.width)
                }
            }
        }
    }

    DelegateModel {
        id: visualModel
        filterOnGroup: "visibleItems"
        model: LogModel

        delegate: Rectangle {
            height: 60
            width: listView.width
            border.width: 1
            border.color: CSC.Style.lightGrey

            RowLayout {
                spacing: 30
                anchors.fill: parent
                anchors.leftMargin: CSC.Style.padding

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
                }

                Text {
                    id: messageLabel
                    text: message[0]
                    wrapMode: Text.Wrap
                    font.pointSize: 15
                    Layout.fillWidth: true
                }
            }
        }

        groups: [
            DelegateModelGroup {
                id: visibleItems
                name: "visibleItems"
                includeByDefault: true

                onChanged: {
                    if (count > listView.amountVisible) {
                        visibleItems.setGroups(0, 1, "items")
                    }
                }
            }
        ]
    }
}
