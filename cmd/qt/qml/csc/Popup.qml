import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls.Material 2.12
import csc 1.2 as CSC

Popup {
    id: popup
    x: 0
    y: parent.height - popup.height
    modal: true
    height: contentColumn.implicitHeight + topPadding + bottomPadding
    topPadding: background.border.width + CSC.Style.padding
    bottomPadding: background.border.width + CSC.Style.padding
    leftPadding: background.border.width
    rightPadding: background.border.width + CSC.Style.padding
    leftMargin: CSC.Style.padding
    rightMargin: CSC.Style.padding

    property string errorMessage: ""
    property color mainColor: CSC.Style.red

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
                icon.source: "qrc:/qml/images/x-circle-fill.svg"
                icon.color: mainColor
                icon.width: diameter
                icon.height: diameter
                enabled: false
                Layout.preferredWidth: 3 * diameter
                Layout.alignment: Qt.AlignVCenter

                property real diameter: CSC.Style.padding

                background: Rectangle {
                    color: "transparent"
                }
            }

            Text {
                id: errorText
                text: popup.errorMessage
                verticalAlignment: Text.AlignVCenter
                wrapMode: Text.Wrap
                font.pixelSize: 15
                Layout.fillWidth: true
            }
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
        border.width: 2
        border.color: mainColor
        implicitWidth: popup.parent.width
        implicitHeight: popup.height
        radius: 8
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