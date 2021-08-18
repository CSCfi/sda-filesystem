import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick 2.13
import csc 1.0 as CSC

TextField {
    id: textfield
    bottomInset: 8
    leftPadding: 8
    rightPadding: 8
    selectByMouse: true
    mouseSelectionMode: TextInput.SelectWords

    background: Rectangle {
        id: bg
        color: CSC.Style.lightGreyBlue
        border.width: textfield.focus ? 2 : 1
        border.color: textfield.focus ? CSC.Style.primaryColor : CSC.Style.lineGray
        radius: 5
    }
}