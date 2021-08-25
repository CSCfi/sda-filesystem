import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls.Material 2.12
import csc 1.0 as CSC

Popup {
    id: popup
    x: 0
    y: parent.height - popup.height
    height: contentColumn.implicitHeight
    topPadding: background.border.width
    bottomPadding: background.border.width
    rightPadding: closePopup.width + background.border.width + 3
    leftPadding: background.border.width
    modal: false
    focus: modal
    rightMargin: margin
    leftMargin: margin
    closePolicy: Popup.NoAutoClose

    property string errorTextContent: ""
    property string errorTextClarify: ""
    property int margin: 20
    property bool isError: true
    property color mainColor: isError ? CSC.Style.red : CSC.Style.warningOrange
        
    ColumnLayout {
        id: contentColumn
        spacing: 0
        anchors.right: parent.right
        anchors.left: parent.left

        RowLayout {
            spacing: 0
            Layout.fillWidth: true
            Layout.topMargin: popup.margin
            Layout.bottomMargin: popup.margin

            // This is a button only so that the svg is easier to color
            RoundButton {
                id: errorIcon
                padding: 0
                icon.source: isError ? "qrc:/qml/images/x-circle-fill.svg" : "qrc:/qml/images/exclamation-triangle-fill.svg"
                icon.color: mainColor
                icon.width: diameter
                icon.height: diameter
                enabled: false
                Layout.preferredWidth: 3 * diameter
                Layout.alignment: Qt.AlignVCenter

                property real diameter: popup.margin

                background: Rectangle {
                    color: "transparent"
                }
            }

            Text {
                id: errorText
                text: popup.errorTextContent
                verticalAlignment: Text.AlignVCenter
                wrapMode: Text.Wrap
                font.weight: Font.Medium
                Layout.fillWidth: true
            }
        }

        Text {
            text: "Error"
            maximumLineCount: 1
            visible: rectClarify.visible
            Layout.leftMargin: errorIcon.width
        }

        Rectangle {
            id: rectClarify
            color: CSC.Style.lightGreyBlue
            border.width: 1
            border.color: CSC.Style.lineGray
            visible: isError && errorClarify.text != ""
            Layout.preferredHeight: 70 
            Layout.fillWidth: true
            Layout.leftMargin: errorIcon.width
            Layout.bottomMargin: popup.margin

            ScrollView {
                clip: true
                anchors.fill: parent

                Text {
                    id: errorClarify
                    text: popup.errorTextClarify
                    font: QmlBridge.fixedFont
                    padding: 5
                }
            }
        }

         // This is inside ColumnLayout beacuse Popup cannot have states
        states: [
            State {
                name: "centered"
                when: errorClarify.text != ""
                PropertyChanges {
                    target: popup
                    modal: true
                    parent: Overlay.overlay
                    x: Math.round((parent.width - width) / 2)
                    y: Math.round((parent.height - height) / 2)
                    closePolicy: Popup.CloseOnEscape | Popup.CloseOnPressOutside
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

        RoundButton {
            id: closePopup
            text: "\u2573"
            Material.foreground: mainColor
            Material.background: "transparent"
            anchors.right: parent.right
            height: 30
            width: height
            
            onClicked: popup.close()
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