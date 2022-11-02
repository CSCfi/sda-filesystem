import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls.Material 2.12
import csc 1.3 as CSC

Popup {
    id: popup
    x: 0
    y: parent.height - popup.height
    modal: true
    height: contentColumn.implicitHeight + topPadding + bottomPadding
    topPadding: borderWidth + CSC.Style.padding
    bottomPadding: borderWidth + CSC.Style.padding
    leftPadding: borderWidth + 0.5 * CSC.Style.padding
    rightPadding: borderWidth + CSC.Style.padding
    leftMargin: CSC.Style.padding
    rightMargin: CSC.Style.padding

    property string errorMessage: ""
    property string additionalText: ""
    property color mainColor: CSC.Style.red
    property int borderWidth: 2

    default property alias content: extraContent.data
    property alias state: contentColumn.state
        
    ColumnLayout {
        id: contentColumn
        spacing: 0
        state: popup.content.length != 0 ? "centered" : ""
        anchors.right: parent.right
        anchors.left: parent.left

        RowLayout {
            spacing: 0
            Layout.fillWidth: true

            // This is a button only so that the svg is easier to color
            RoundButton {
                id: errorIcon
                padding: 0
                icon.source: popup.mainColor == CSC.Style.red ? "qrc:/qml/images/x-circle-fill.svg" : "qrc:/qml/images/warning_black.svg"
                icon.color: mainColor
                icon.width: diameter
                icon.height: diameter
                enabled: false
                Layout.preferredWidth: 2 * diameter
                Layout.alignment: Qt.AlignVCenter

                property real diameter: 25

                background: Rectangle {
                    color: "transparent"
                }
            }

            Label {
                id: errorText
                text: popup.errorMessage
                color: CSC.Style.grey
                verticalAlignment: Text.AlignVCenter
                wrapMode: Text.Wrap
                font.pixelSize: popup.additionalText != "" ? 17 : 15
                font.weight: popup.additionalText != "" ? Font.Bold : Font.Medium
                Layout.fillWidth: true
            }
        }

        Label {
            text: popup.additionalText
            color: CSC.Style.grey
            wrapMode: Text.Wrap
            visible: popup.additionalText != ""
            topPadding: 0.5 * CSC.Style.padding
            bottomPadding: 0.5 * CSC.Style.padding
            font.pixelSize: 15
            font.weight: Font.Medium
            Layout.fillWidth: true
            Layout.leftMargin: errorIcon.width
        }

        Item {
            id: extraContent
            Layout.leftMargin: errorIcon.width
            Layout.preferredHeight: childrenRect.height
            Layout.fillWidth: true
        }

         // This is inside ColumnLayout beacuse Popup cannot have states
        states: [
            State {
                name: "centered"
                PropertyChanges {
                    target: popup
                    parent: Overlay.overlay
                    anchors.centerIn: Overlay.overlay
                }
            }
        ]
    }

    background: Rectangle {
        color: mainColor
        layer.enabled: true
        implicitWidth: popup.parent.width
        implicitHeight: popup.height
        radius: 8

        Rectangle {
            width: 0.5 * parent.width
            height: parent.height
            color: "white"
            radius: parent.radius
            border.color: mainColor
            border.width: popup.borderWidth
            anchors.right: parent.right
        }

        Rectangle {
            color: "white"
            width: parent.width - CSC.Style.padding - 2 * popup.borderWidth
            height: parent.height - 2 * popup.borderWidth
            anchors.centerIn: parent
        }
    }
    
    enter: Transition {
        ParallelAnimation {
            NumberAnimation { 
                property: "y"; 
                from: contentColumn.state == "" ? popup.parent.height : popup.y; 
                to: contentColumn.state == "" ? popup.parent.height - popup.height : popup.y; 
                duration: 500; 
                easing.type: Easing.OutQuad 
            }
            NumberAnimation { property: "opacity"; from: 0.0; to: 1.0; duration: contentColumn.state == "" ? 500 : 100; }
        }
    }

    exit: Transition {
        ParallelAnimation {
            NumberAnimation { 
                property: "y"; 
                from: contentColumn.state == "" ? popup.parent.height - popup.height : popup.y; 
                to: contentColumn.state == "" ? popup.parent.height : popup.y; 
                duration: 500; 
                easing.type: Easing.InQuad 
            }
            NumberAnimation { property: "opacity"; from: 1.0; to: 0.0; duration: contentColumn.state == "" ? 500 : 100; }
        }
    }
}