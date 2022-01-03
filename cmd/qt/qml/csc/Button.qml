import QtQuick 2.13
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

    Material.foreground: !button.enabled ? disableForeground : foregroundColor
    Material.accent: foregroundColor

    property bool loading: false
    property bool outlined: false
    property color mainColor: CSC.Style.primaryColor
    property color backgroundColor: outlined ? "white" : mainColor
    property color foregroundColor: outlined ? mainColor : "white"
    property color hoveredColor: outlined ? "#E2ECEE" : "#61958D"
    property color pressedColor: outlined ? "#E8F0F1" : "#9BBCB7"
    property color disableBackgound: loading ? backgroundColor : "#E8E8E8"
    property color disableForeground: "#8C8C8C"

    background: Rectangle {
        radius: 4
        border.width: outlined ? 2 : 0
        border.color: button.loading ? foregroundColor : (!button.enabled ? disableForeground : (button.pressed ? "#779DA7" : mainColor))
        color: !button.enabled ? disableBackgound : (button.pressed ? pressedColor : (button.hovered ? hoveredColor : backgroundColor))
    }

    BusyIndicator {
        id: busy
        running: button.loading
        anchors.fill: parent
        anchors.centerIn: parent
        anchors.margins: 5
    }

    MouseArea {
        id: mouseArea
        cursorShape: Qt.PointingHandCursor
        acceptedButtons: Qt.NoButton
        anchors.fill: parent
    }
}