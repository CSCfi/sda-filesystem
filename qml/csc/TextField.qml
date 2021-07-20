import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
//import QtQuick.Controls.Material 2.12
import QtQuick 2.13
import csc 1.0 as CSC

TextField {
    id: textfield
    bottomInset: 8
    leftPadding: 8
    rightPadding: 8
    selectByMouse: true
    mouseSelectionMode: TextInput.SelectWords
    Layout.alignment: Qt.AlignCenter
    Layout.fillWidth: true

    //Material.accent: CSC.Style.primaryColor

    background: Rectangle {
        id: bg
        color: CSC.Style.lightGreyBlue
        border.width: textfield.focus ? 2 : 1
        border.color: textfield.focus ? CSC.Style.primaryColor : "#707070"
        radius: 5
    }
}