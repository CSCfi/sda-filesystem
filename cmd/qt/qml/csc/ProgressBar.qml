import QtQuick 2.13
import QtQuick.Controls 2.13
import csc 1.2 as CSC

ProgressBar {
    id: bar

    background: Rectangle {
        implicitHeight: 15
        color: CSC.Style.lightGrey
        radius: height * 0.5
    }

    contentItem: Rectangle {
        clip: true
        radius: height * 0.5 
        color: "transparent"
        border.color: CSC.Style.lightGrey
        border.width: 3
        anchors.fill: parent

        Pane {
            z: -1
            width: parent.border.width
            height: parent.height
            anchors.left: parent.left
        }

        Pane {
            z: -1
            width: parent.border.width
            height: parent.height
            anchors.right: parent.right
        }

        Rectangle {
            id: rect
            z: -2
            width: !bar.indeterminate ? bar.visualPosition * parent.width : parent.width * 0.7
            radius: height * 0.5
            color: CSC.Style.turquoise
            state: bar.indeterminate ? "left" : ""
            anchors.top: parent.top
            anchors.bottom: parent.bottom
            anchors.margins: parent.border.width

            Timer {
                interval: 2000
                running: bar.indeterminate && bar.visible
                repeat: true
                triggeredOnStart: true
                onTriggered: rect.state = "right"
            }

            states: [
                State {
                    name: "left"
                    AnchorChanges { target: rect; anchors.right: rect.parent.left }
                },
                State {
                    name: "right"
                    AnchorChanges { target: rect; anchors.left: rect.parent.right }
                }
            ]

            transitions: Transition {
                to: "right"
                AnchorAnimation { duration: 1500 }

                onRunningChanged: {
                    if (rect.state == "right" && !running) {
                        rect.state = "left"     
                    }
                }
            }
        }
    }
}