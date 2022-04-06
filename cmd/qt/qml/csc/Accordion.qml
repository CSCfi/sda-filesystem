import QtQuick 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls 2.13
import QtQuick.Controls.Material 2.12
import csc 1.0 as CSC

ColumnLayout {
    id: accordion
    spacing: 0

    property string heading
    property color backgroundColor: enabled ? CSC.Style.lightBlue : "#E8E8E8"
    property color textColor: enabled ? CSC.Style.primaryColor : "#8C8C8C"
    property bool open: false
    property bool success: false
    property bool loading: false

    default property alias content: extraContent.data

    onSuccessChanged: {
        if (success) {
            toggle.state = "done"
            area.enabled = false
            extraContent.visible = false
        }
    }

    Rectangle {
        id: bkg
        color: backgroundColor
        implicitHeight: 45
        radius: 5

        Layout.preferredHeight: implicitHeight
        Layout.fillWidth: true

        RowLayout {
            anchors.fill: parent
            anchors.leftMargin: CSC.Style.padding
            anchors.rightMargin: CSC.Style.padding

            Text {
                text: heading
                color: textColor
                maximumLineCount: 1
                font.weight: Font.Medium
                font.pixelSize: 0.33 * bkg.implicitHeight
                Layout.alignment: Qt.AlignVCenter
                Layout.fillWidth: true
            }

            Item {
                height: parent.height
                Layout.preferredWidth: toggle.width

                CSC.Toggle {
                    id: toggle
                    height: 0.45 * bkg.height
                    opacity: busy.running ? 0.5 : 1
                    anchors.verticalCenter: parent.verticalCenter
                }

                BusyIndicator {
                    id: busy
                    running: accordion.loading
                    anchors.fill: parent
                    anchors.centerIn: toggle
                }
            }
        }

        MouseArea {
            id: area
            cursorShape: Qt.PointingHandCursor
            anchors.fill: parent

            onClicked: {
                extraContent.visible = !extraContent.visible
                accordion.open = extraContent.visible // Only want to trigger onOpenChanged when accordion is clicked
            }
        }
    }

    Item {
        id: extraContent
        visible: false
        Layout.preferredHeight: childrenRect.height
        Layout.fillWidth: true
    }
}