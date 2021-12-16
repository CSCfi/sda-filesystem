import QtQuick 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls 2.13
import QtQuick.Controls.Material 2.12
import csc 1.0 as CSC

ColumnLayout {
    id: accordion
    spacing: 0

    property string heading
    property color backgroundColor: "#DBE7E9"
    property color textColor: CSC.Style.primaryColor
    property bool opened: extraContent.visible
    property bool success: false

    default property alias content: extraContent.data

    function hide() {
        extraContent.visible = false
        toggle.toggleState()
    }

    onSuccessChanged: {
        if (success) {
            toggle.done()
            area.enabled = false
            extraContent.visible = false
        }
        console.log(success);
    }

    Rectangle {
        id: bkg
        color: backgroundColor
        implicitHeight: 40
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
                font.pointSize: 0.33 * bkg.implicitHeight
                Layout.alignment: Qt.AlignVCenter
                Layout.fillWidth: true
            }

            CSC.Toggle {
                id: toggle
                width: 40
            }
        }

        MouseArea {
            id: area
            anchors.fill: parent
            onClicked: { 
                extraContent.visible = !extraContent.visible
                toggle.toggleState()
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