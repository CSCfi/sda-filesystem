import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Controls.Material 2.12
import csc 1.0 as CSC

Button {
    id: button
    hoverEnabled: true
    padding: 15
    Material.foreground: !button.enabled ? disableForeground : foregroundColor

    property bool outlined: false
    property color backgroundColor: outlined ? "white" : CSC.Style.primaryColor
    property color foregroundColor: outlined ? CSC.Style.primaryColor : "white"
    property color hoveredColor: outlined ? "#E2ECEE" : "#61958D"
    property color pressedColor: outlined ? "#E8F0F1" : "#9BBCB7"
    property color disableBackgound: "#E8E8E8"
    property color disableForeground: "#8C8C8C"

    background: Rectangle {
        radius: 4
        border.width: outlined ? 2 : 0
        border.color: !button.enabled ? disableForeground : (button.pressed ? "#779DA7" : CSC.Style.primaryColor)
        color: !button.enabled ? disableBackgound : (button.pressed ? pressedColor : (button.hovered ? hoveredColor : backgroundColor))
    }
}