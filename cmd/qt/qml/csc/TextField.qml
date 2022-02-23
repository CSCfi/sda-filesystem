import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick 2.13
import csc 1.0 as CSC

TextField {
    id: textfield
    topPadding: 8
    leftPadding: topPadding
    rightPadding: topPadding
    bottomPadding: errorVisible ? topPadding + bottomInset : topPadding
    bottomInset: errorVisible ? errorRow.height : 0
    selectByMouse: true
    mouseSelectionMode: TextInput.SelectWords

    property string errorText
    property bool errorVisible: false

    background: Rectangle {
        id: bg
        border.width: textfield.activeFocus ? 2 : 1
        border.color: textfield.activeFocus ? CSC.Style.primaryColor : CSC.Style.grey
        radius: 5
    }

    RowLayout {
        id: errorRow
        visible: errorVisible
        anchors.bottom: parent.bottom

        RoundButton {
            id: error401
            padding: 0
            icon.source: "qrc:/qml/images/x-circle-fill.svg"
            icon.color: CSC.Style.red
            icon.width: 15
            icon.height: 15
            enabled: false
            Layout.alignment: Qt.AlignVCenter

            background: Rectangle {
                color: "transparent"
            }
        }

        Text {
            text: errorText
            color: CSC.Style.grey
            height: contentHeight
            font.pixelSize: 12
            Layout.fillWidth: true
        }
    }
}