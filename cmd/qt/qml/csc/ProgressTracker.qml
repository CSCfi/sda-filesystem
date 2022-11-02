import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import csc 1.3 as CSC

Control {
    id: tracker
    leftPadding: 0.5 * repeater.textMaxWidth + 2 * CSC.Style.padding
    rightPadding: 0.5 * repeater.textMaxWidth + 2 * CSC.Style.padding
    topPadding: 2 * CSC.Style.padding

    property int progressIndex: 0
    property var model: []

    contentItem: RowLayout {
        spacing: 0

        Repeater {
            id: repeater
            model: tracker.model

            property real textMaxWidth: 0

            Control {
                implicitHeight: childrenRect.height
                padding: 0
                Layout.preferredWidth: (index == 0) ? circle.width : -1
                Layout.fillWidth: (index == 0) ? false : true

                Component.onCompleted: repeater.textMaxWidth = Math.max(info.contentWidth, repeater.textMaxWidth)

                Rectangle {
                    height: 2
                    color: info.color
                    anchors.right: parent.right
                    anchors.left: parent.left
                    anchors.verticalCenter: circle.verticalCenter
                }

                Rectangle {
                    id: circle
                    width: 20
                    height: width
                    radius: 0.5 * width
                    border.color: info.color
                    border.width: 2
                    anchors.right: parent.right
                    anchors.top: parent.top

                    Rectangle {
                        height: parent.height - 2 * anchors.margins
                        width: height
                        radius: 0.5 * height
                        color: CSC.Style.primaryColor
                        visible: (index == tracker.progressIndex) ? true : false
                        anchors.margins: 4
                        anchors.left: parent.left
                        anchors.verticalCenter: parent.verticalCenter
                    }

                    // This is a button only so that the svg is easier to color
                    RoundButton {
                        padding: 0
                        icon.source: "qrc:/qml/images/check-circle-fill.svg"
                        icon.color: CSC.Style.primaryColor
                        icon.width: circle.width
                        icon.height: circle.height
                        enabled: false
                        visible: (index < tracker.progressIndex) ? true : false
                        anchors.centerIn: circle

                        background: Rectangle {
                            color: "white"
                        }
                    }
                }

                Label {
                    id: info
                    text: modelData
                    color: (index <= tracker.progressIndex) ? CSC.Style.primaryColor : CSC.Style.grey
                    font.pixelSize: 12
                    anchors.top: circle.bottom
                    anchors.topMargin: 8
                    anchors.horizontalCenter: circle.horizontalCenter
                }
            }
        }
    }
}