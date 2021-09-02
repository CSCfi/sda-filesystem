import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls.Material 2.12
import csc 1.0 as CSC

Popup {
    id: popup
    x: 0
    y: parent.height - popup.height
    height: contentColumn.implicitHeight + topPadding + bottomPadding
    topPadding: background.border.width + CSC.Style.padding
    bottomPadding: background.border.width + CSC.Style.padding
    leftPadding: background.border.width
    rightPadding: closePopup.width + background.border.width + 4
    leftMargin: CSC.Style.padding
    rightMargin: CSC.Style.padding
    modal: false
    focus: modal
    closePolicy: Popup.NoAutoClose

    property string errorTextContent: ""
    property string errorTextClarify: ""
    property int type: LogLevel.Error
    property color mainColor: {
        if (popup.type == LogLevel.Error) {
            return CSC.Style.red
        } else if (popup.type == LogLevel.Warning) {
            return CSC.Style.warningOrange
        } else if (popup.type == LogLevel.Info) {
            return CSC.Style.primaryColor
        } else {
            return "transparent"
        }
    }

    default property alias content: extraContent.data
        
    ColumnLayout {
        id: contentColumn
        spacing: 0
        anchors.right: parent.right
        anchors.left: parent.left

        RowLayout {
            spacing: 0
            Layout.fillWidth: true

            // This is a button only so that the svg is easier to color
            RoundButton {
                id: errorIcon
                padding: 0
                icon.source: {
                    if (popup.type == LogLevel.Error) {
                        return "qrc:/qml/images/x-circle-fill.svg"
                    } else if (popup.type == LogLevel.Warning) {
                        return "qrc:/qml/images/exclamation-triangle-fill.svg"
                    } else if (popup.type == LogLevel.Info) {
                        return "qrc:/qml/images/info-circle-fill.svg"
                    }
                }
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
                text: popup.errorTextContent
                verticalAlignment: Text.AlignVCenter
                wrapMode: Text.Wrap
                font.pointSize: 15
                Layout.fillWidth: true
            }
        }

        Text {
            text: "Error"
            maximumLineCount: 1
            visible: rectClarify.visible
            Layout.leftMargin: errorIcon.width
            Layout.topMargin: CSC.Style.padding
        }

        Rectangle {
            id: rectClarify
            color: CSC.Style.lightGreyBlue
            border.width: 1
            border.color: CSC.Style.lineGray
            visible: errorClarify.text != ""
            Layout.preferredHeight: 80 
            Layout.fillWidth: true
            Layout.leftMargin: errorIcon.width

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

    function toCentered() {
        contentColumn.state = "centered"
    }

    background: Rectangle {
        border.width: 2
        border.color: mainColor
        implicitWidth: popup.parent.width
        implicitHeight: popup.height
        radius: 8

        RoundButton {
            id: closePopup
            padding: 0
            icon.source: "qrc:/qml/images/x-lg.svg"
            icon.color: mainColor
            icon.width: width / 3
            icon.height: height / 3
            width: 25
            height: 25
            topInset: 0
            bottomInset: 0
            rightInset: 0
            leftInset: 0
            visible: contentColumn.state == ""
            anchors.top: parent.top
            anchors.right: parent.right
            anchors.margins: 4

            Material.background: "transparent"

            onClicked: popup.close()

            MouseArea {
                cursorShape: Qt.PointingHandCursor
                acceptedButtons: Qt.NoButton
                anchors.fill: parent
            }
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