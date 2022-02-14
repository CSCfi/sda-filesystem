import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQml.Models 2.13
import QtQuick.Controls.Material 2.12
import Qt.labs.qmlmodels 1.0
import QtQuick.Dialogs 1.3
import csc 1.0 as CSC

ListView {
    id: listView
    width: parent.width
    height: contentHeight
    implicitWidth: listView.headerItem.implicitWidth
    interactive: false
    //boundsBehavior: Flickable.StopAtBounds
    verticalLayoutDirection: ListView.BottomToTop

    property int amountVisible: 5
    property int page: 1
    property int maxPages: Math.max(1, Math.ceil(modelSource.count / amountVisible))
    property variant modelSource
    property Component delegateSource

    onPageChanged: {
        visibleItems.setGroups(0, visibleItems.count, "items")
        var ceilItemCount = page * amountVisible 
        var visible = amountVisible
        if (ceilItemCount > modelSource.count) {
            visible -= (ceilItemCount - modelSource.count)
            ceilItemCount = modelSource.count
        }
        visualModel.items.setGroups(modelSource.count - ceilItemCount, visible, "visibleItems")
    }

    header: Rectangle {
        height: 40
        width: listView.width
        implicitWidth: pageCount.width + 10 * height
        border.width: 1
        border.color: CSC.Style.lightGrey

        Text {
            text: "No logs available"
            visible: modelSource.count == 0
            verticalAlignment: Text.AlignVCenter
            font.pointSize: 15
            anchors.fill: parent
            anchors.leftMargin: CSC.Style.padding
        }

        Row {
            spacing: 20
            visible: modelSource.count > 0
            height: parent.height
            leftPadding: CSC.Style.padding

            Material.foreground: CSC.Style.primaryColor

            RowLayout {
                id: pageCount
                spacing: 10
                height: parent.height

                Text {
                    text: "Items per page: "
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

            Text {
                text: firstIdx + " - " + lastIdx + " of " + modelSource.count + " items"
                height: parent.height
                verticalAlignment: Text.AlignVCenter

                property int firstIdx: (listView.page - 1) * listView.amountVisible + 1
                property int lastIdx: {
                    if (modelSource.count < listView.amountVisible) {
                        return modelSource.count
                    } else {
                        return firstIdx + listView.amountVisible - 1
                    }
                }
            }

            Text {
                text: listView.page + " of " + listView.maxPages + " pages"
                height: parent.height
                verticalAlignment: Text.AlignVCenter 
            }
        }

        Row {
            id: pageSelect
            visible: modelSource.count > 0 && listView.maxPages > 1
            height: parent.height
            anchors.right: parent.right

            ToolButton {
                id: pageLeft
                icon.source: "qrc:/qml/images/chevron-left.svg"
                height: parent.height
                width: height

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

            ListView {
                id: pageList
                height: parent.height
                width: contentWidth
                orientation: ListView.Horizontal 

                model: listView.maxPages < 7 ? listView.maxPages : 7
                delegate: ToolButton {
                    text: {
                        switch (index) {
                            case 0:
                                return 1
                            case 1:
                                if (listView.page < 4 || listView.maxPages < 7) {
                                    return 2
                                } else {
                                    return ""
                                }
                            case 2:
                                if (listView.page < 4 || listView.maxPages < 7) {
                                    return 3
                                } else if (listView.page > listView.maxPages - 3) {
                                    return listView.maxPages - 4
                                } else {
                                    return listView.page - 1
                                }
                            case 3:
                                if (listView.page < 4 || listView.maxPages < 7) {
                                    return 4
                                } else if (listView.page > listView.maxPages - 3) {
                                    return listView.maxPages - 3
                                } else {
                                    return listView.page
                                }
                            case 4:
                                if (listView.page < 4 || listView.maxPages < 7) {
                                    return 5
                                } else if (listView.page > listView.maxPages - 3) {
                                    return listView.maxPages - 2
                                } else {
                                    return listView.page + 1
                                }
                            case 5:
                                if (listView.maxPages < 7) {
                                    return 6
                                } else if (listView.page > listView.maxPages - 3) {
                                    return listView.maxPages - 1
                                } else {
                                    return ""
                                }
                            case 6:
                                return listView.maxPages
                        }
                    }
                    height: pageList.height
                    width: height
                    enabled: text != ""
                    icon.source: (text == "") ? "qrc:/qml/images/three-dots.svg" : ""

                    Material.foreground: parseInt(text, 10) != listView.page ? CSC.Style.grey : CSC.Style.primaryColor

                    onClicked: listView.page =  parseInt(text, 10)

                    MouseArea {
                        cursorShape: Qt.PointingHandCursor
                        acceptedButtons: Qt.NoButton
                        anchors.fill: parent
                    }
                }
            }

            ToolButton {
                id: pageRight
                icon.source: "qrc:/qml/images/chevron-right.svg"
                height: parent.height
                width: height

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
    }

    model: DelegateModel {
        id: visualModel
        model: listView.modelSource
        delegate: listView.delegateSource
        filterOnGroup: "visibleItems"
        groups: [
            DelegateModelGroup {
                id: visibleItems
                name: "visibleItems"
                includeByDefault: true

                onChanged: {
                    if (count > table.amountVisible) {
                        visibleItems.setGroups(0, 1, "items")
                    }
                }
            }
        ]
    }
}
