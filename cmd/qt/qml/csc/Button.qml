import QtQuick 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls 2.13
import QtQuick.Controls.Material 2.12
import csc 1.0 as CSC

Button {
    id: button
    hoverEnabled: true
    topPadding: 15
    bottomPadding: 15
    rightPadding: 25
    leftPadding: 25
    topInset: 0
    bottomInset: 0
    enabled: !loading
    implicitWidth: implicitContentWidth + (loading ? height - busy.anchors.margins : 0) + leftPadding + rightPadding

    Material.accent: foregroundColor

    property bool loading: false
    property bool outlined: false
    property color mainColor: CSC.Style.primaryColor
    property color backgroundColor: outlined ? "white" : mainColor
    property color foregroundColor: outlined ? mainColor : "white"
    property color hoveredColor: outlined ? "#E2ECEE" : "#61958D"
    property color pressedColor: outlined ? "#E8F0F1" : "#9BBCB7"
    property color disabledBackgound: loading ? backgroundColor : "#E8E8E8"
    property color disabledForeground: loading ? foregroundColor : "#8C8C8C"

    background: Rectangle {
        radius: 4
        border.width: outlined ? 2 : 0
        border.color: !button.enabled ? disabledForeground : (button.pressed ? "#779DA7" : mainColor)
        color: !button.enabled ? disabledBackgound : (button.pressed ? pressedColor : (button.hovered ? hoveredColor : backgroundColor))
    }

    contentItem: Text {
        text: button.text
        font: button.font
        padding: 0
        color: button.enabled ? foregroundColor : disabledForeground
        horizontalAlignment: loading ? Text.AlignLeft : Text.AlignHCenter
        verticalAlignment: Text.AlignVCenter
        elide: Text.ElideRight
    }

    indicator: BusyIndicator {
        id: busy
        running: button.loading
        visible: running
        padding: 0
        anchors.top: button.top
        anchors.bottom: button.bottom
        anchors.right: button.right
        anchors.margins: 10
    }

    MouseArea {
        id: mouseArea
        cursorShape: Qt.PointingHandCursor
        acceptedButtons: Qt.NoButton
        anchors.fill: parent
    }
}