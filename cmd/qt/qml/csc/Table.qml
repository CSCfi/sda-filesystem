import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQml.Models 2.13
import QtQuick.Controls.Material 2.12
import Qt.labs.qmlmodels 1.0
import QtQuick.Dialogs 1.3
import csc 1.2 as CSC

ListView {
    id: listView
    implicitHeight: contentHeight
    implicitWidth: listView.headerItem.implicitWidth
    interactive: false
    verticalLayoutDirection: ListView.BottomToTop

    Material.foreground: CSC.Style.grey

    property variant modelSource
    property Component delegateSource
    property int rowCount: visualModel.items.count
    property int amountVisible: 5
    property int page: 1
    property int maxPages: Math.ceil(rowCount / amountVisible)

    Keys.onRightPressed: {
        if (listView.rowCount != 0) {
            headerItem.changePageRight()
        }
    }
    Keys.onLeftPressed: {
        if (listView.rowCount != 0) {
            headerItem.changePageLeft()
        }
    }

    onPageChanged: selectVisible()
    onRowCountChanged: selectVisible()
    onMaxPagesChanged: {
        if (page > maxPages) {
            var topLog = (listView.maxPages - 1) * listView.amountVisible
            listView.page = Math.floor(topLog / listView.amountVisible) + 1
        }
    }

    function selectVisible() {
        if (visibleItems.count > 0) {
            visibleItems.remove(0, visibleItems.count)
        }
        var ceilItemCount = page * amountVisible 
        var visible = amountVisible
        if (ceilItemCount > rowCount) {
            visible -= (ceilItemCount - rowCount)
            ceilItemCount = rowCount
        }
        if (visible > 0) {
            visualModel.items.addGroups(rowCount - ceilItemCount, visible, "visibleItems")
        }
    }

    header: Rectangle {
        height: 40
        width: listView.width
        implicitWidth: pageCount.width + 10 * modelButton.implicitWidth
        border.width: 1
        border.color: CSC.Style.lightGrey

        Label {
            text: "No " + listView.objectName + " available"
            visible: listView.rowCount == 0
            verticalAlignment: Text.AlignVCenter
            font.pixelSize: 14
            anchors.fill: parent
            anchors.leftMargin: CSC.Style.padding
        }

        ToolButton {
            id: modelButton
            text: "99999"
            visible: false
            enabled: false
        }

        function changePageLeft() {
            pageLeft.clicked()
        }

        function changePageRight() {
            pageRight.clicked()
        }

        Row {
            id: leftRow
            spacing: CSC.Style.padding
            visible: listView.rowCount > 0
            height: parent.height

            RowLayout {
                id: pageCount
                spacing: 10
                height: parent.height

                Label {
                    text: "Items per page: "
                    leftPadding: CSC.Style.padding
                    font.pixelSize: 12
                }

                ToolButton {
                    id: perPage
                    text: listView.amountVisible + "  "
                    font.pixelSize: 15
                    icon.source: menu.visible ? "qrc:/qml/images/chevron-up.svg" : "qrc:/qml/images/chevron-down.svg"
                    icon.width: 20
                    icon.height: 20
                    LayoutMirroring.enabled: true
                    Layout.fillHeight: true

                    Material.foreground: CSC.Style.primaryColor

                    Component.onCompleted: Layout.preferredWidth = 1.5 * implicitWidth

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

                    onClicked: menu.visible = !menu.visible

                    Menu {
                        id: menu

                        property bool down: true

                        onAboutToShow: {
                            if (mapToItem(null, 0, height).y > window.height) {
                                down = false
                                y = -height
                            } else {
                                down = true
                                y = parent.height - 1
                            }
                        }

                        onYChanged: {
                            if (down && visible && y < parent.height - 1) {
                                down = false
                                y = -height
                            }
                        }

                        background: Rectangle {
                            implicitWidth: perPage.width
                            color: "white"
                            border.width: 1
                            border.color: CSC.Style.lightGrey
                        }

                        Repeater {
                            model: 4
                            MenuItem {
                                text: amount //Array.from(Array(4), (_,i)=> 5 + 5 * i)

                                property int amount
                                
                                Component.onCompleted: amount = (index + 1) * listView.amountVisible
                                onTriggered: {
                                    var topLog = (listView.page - 1) * listView.amountVisible
                                    listView.amountVisible = amount
                                    var newPage = Math.floor(topLog / amount) + 1
                                    if (newPage != listView.page) {
                                        listView.page = newPage
                                    } else {
                                        selectVisible()
                                    }
                                }
                            }
                        }
                    }
                }
            }

            Label {
                text: firstIdx + " - " + lastIdx + " of " + listView.rowCount + " items"
                height: parent.height
                verticalAlignment: Text.AlignVCenter
                font.pixelSize: 12
                opacity: (rightRow.x + whichPageText.width - CSC.Style.padding > leftRow.width) ? 1.0 : 0.0

                property int firstIdx: (listView.page - 1) * listView.amountVisible + 1
                property int lastIdx: Math.min(firstIdx + listView.amountVisible - 1, listView.rowCount)
            }
        }

        Row {
            id: rightRow
            visible: listView.rowCount > 0 && listView.maxPages > 1
            height: parent.height
            anchors.right: parent.right

            Label {
                id: whichPageText
                text: listView.page + " of " + listView.maxPages + " pages"
                height: parent.height
                verticalAlignment: Text.AlignVCenter 
                rightPadding: CSC.Style.padding
                leftPadding: CSC.Style.padding
                font.pixelSize: 12
                opacity: (rightRow.x > leftRow.width) ? 1.0 : 0.0
            }

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

                model: (listView.maxPages < 7) ? listView.maxPages : 7
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
                    width: Math.max(height, implicitWidth)
                    font.weight: Font.DemiBold
                    icon.source: (text == "") ? "qrc:/qml/images/three-dots.svg" : ""

                    Material.foreground: parseInt(text, 10) != listView.page ? CSC.Style.grey : CSC.Style.primaryColor

                    onClicked: {
                        if (text == "") {
                            var high = parseInt(pageList.itemAtIndex(index + 1).text)
                            var low = parseInt(pageList.itemAtIndex(index - 1).text)
                            listView.page = Math.floor((high + low) / 2)
                        } else {
                            listView.page =  parseInt(text, 10)
                        }
                    }

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
                includeByDefault: false
            }
        ]
    }
}
