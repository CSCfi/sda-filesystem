import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Controls.Material 2.12
import csc 1.0 as CSC

Rectangle {
    id: toggle
    height: height
    radius: 0.5 * height
    color: "green"
    border.width: 2
    border.color: CSC.Style.grey

    function toggleState(open) {
        if (state != "done") {
            if (open) {
                state = "waiting"
            } else {
                state = ""
            }
        }
    }

    Rectangle {
        id: circle
        height: toggle.height - 10
        width: height
        radius: 0.5 * height
        color: CSC.Style.grey
        anchors.left: parent.left
    }

    states: [
        State {
            name: "waiting"
            AnchorChanges { target: circle; anchors.left: undefined; anchors.right: toggle.right }
            PropertyChanges { target: circle; color: "white" }
            PropertyChanges { target: toggle; color: CSC.Style.warningOrange }
        },
        State {
            name: "done"
            PropertyChanges { target: toggle; color: CSC.Style.primaryColor }
        }
    ]
}