import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Controls.Material 2.12
import csc 1.3 as CSC

Rectangle {
    id: toggle
    width: 2 * height
    radius: 0.5 * height
    color: "transparent"
    border.width: 2
    border.color: CSC.Style.grey

    Rectangle {
        id: circle
        height: toggle.height - 2 * anchors.margins
        width: height
        radius: 0.5 * height
        color: CSC.Style.grey
        anchors.margins: 4
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
    }

    states: [
        State {
            name: "done"
            AnchorChanges { target: circle; anchors.left: undefined; anchors.right: toggle.right }
            PropertyChanges { target: circle; color: "white" }
            PropertyChanges { target: toggle; color: CSC.Style.primaryColor; border.color: color }
        }
    ]
}