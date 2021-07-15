import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls.Material 2.12
import csc 1.0 as CSC

Popup {
    id: popup
    x: 0
    y: parent.height - popup.height
    height: Math.max(errorText.contentHeight, implicitBackgroundHeight)
    modal: false
    focus: false
    padding: 3
    closePolicy: Popup.NoAutoClose
    rightMargin: loginWindow.margins
    leftMargin: loginWindow.margins

    property string errorTextContent

    background: Rectangle {
        border.width: 2
        border.color: CSC.Style.red
        implicitWidth: loginWindow.width - 2 * loginWindow.margins
        implicitHeight: 60
        radius: 8

        RoundButton {
            id: closePopup
            text: "\u2573"
            Material.foreground: CSC.Style.red
            Material.background: "transparent"
            anchors.right: parent.right
            height: parent.implicitHeight * 0.5
            width: height
            
            onClicked: popup.close()
        }
    }

    contentItem: RowLayout {
        spacing: 0

        Rectangle {
            property var diameter: popup.height * 0.25
            radius: diameter * 0.5
            color: CSC.Style.red
            Layout.preferredHeight: diameter
            Layout.preferredWidth: diameter
            Layout.leftMargin: loginWindow.margins
            Layout.rightMargin: loginWindow.margins

            Image {
                source: "qrc:/qml/images/x-lg.svg"
                height: parent.height * 0.6
                fillMode: Image.PreserveAspectFit
                anchors.centerIn: parent
            }
        }
        
        Text {
            id: errorText
            text: popup.errorTextContent
            verticalAlignment: Text.AlignVCenter
            wrapMode: Text.WordWrap
            Layout.rightMargin: closePopup.width + 3
            Layout.fillWidth: true
            Layout.fillHeight: true
        }
    }
    
    enter: Transition {
        ParallelAnimation {
            NumberAnimation { 
                property: "y"; 
                from: loginWindow.height; 
                to: loginWindow.height - popup.height; 
                duration: 500; 
                easing.type: Easing.OutQuad 
            }
            NumberAnimation { property: "opacity"; from: 0.0; to: 1.0; duration: 500; }
        }
    }

    exit: Transition {
        ParallelAnimation {
            NumberAnimation { 
                property: "y"; 
                from: loginWindow.height - popup.height; 
                to: loginWindow.height; 
                duration: 500; 
                easing.type: Easing.InQuad 
            }
            NumberAnimation { property: "opacity"; from: 1.0; to: 0.0; duration: 500; }
        }
    }
}