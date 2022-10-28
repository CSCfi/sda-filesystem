import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick 2.13
import QtQuick.Controls.Material 2.12
import csc 1.2 as CSC

TextField {
    id: textfield
    topPadding: 10
    leftPadding: topPadding
    rightPadding: topPadding
    bottomPadding: extraPadding ? topPadding + bottomInset : topPadding
    bottomInset: extraPadding ? errorRow.height : 0
    selectByMouse: true
    mouseSelectionMode: TextInput.SelectWords

    property string errorText
    property string titleText
    property bool errorVisible: false
    property bool extraPadding: errorText != ""

    background: Rectangle {
        id: bg
        border.width: textfield.activeFocus ? 2 : 1
        border.color: title.color
        radius: 5
    }

    transitions: Transition {
        AnchorAnimation { duration: 300; easing.type: Easing.OutQuart }
        NumberAnimation { duration: 300; properties: "width,font.pixelSize"; easing.type: Easing.OutQuart }
    }

    states: State {
        name: "writing"; when: textfield.activeFocus || textfield.text != ""
        AnchorChanges { target: title; anchors.verticalCenter: textfield.top }
        PropertyChanges { target: title; font.pixelSize: 10; anchors.verticalCenterOffset: 0 }
        PropertyChanges { target: pane; width: title.width }
    }

    Pane {
        id: pane
        width: 0
        height: title.contentHeight
        anchors.verticalCenter: parent.top
        anchors.left: parent.left
        anchors.leftMargin: textfield.leftPadding - title.leftPadding
    }

    Label {
        id: title
        text: textfield.titleText
        leftPadding: 3
        rightPadding: 3
        font.pixelSize: 0.4 * (parent.height - (errorText != "" ? errorRow.height : 0))
        anchors.verticalCenter: parent.verticalCenter
        anchors.verticalCenterOffset: errorText != "" ? -0.5 * errorRow.height : 0
        anchors.left: pane.left

        Material.foreground: textfield.activeFocus ? CSC.Style.primaryColor : CSC.Style.grey
    }

    RowLayout {
        id: errorRow
        visible: errorVisible
        anchors.bottom: parent.bottom

        RoundButton {
            id: error401
            padding: 0
            icon.source: "qrc:/qml/images/x-circle-fill.svg"
            icon.color: CSC.Style.red
            icon.width: 18
            icon.height: 18
            enabled: false

            background: Rectangle {
                color: "transparent"
            }
        }

        Label {
            text: errorText
            color: CSC.Style.grey
            height: contentHeight
            font.pixelSize: 12
            Layout.fillWidth: true
        }
    }
}