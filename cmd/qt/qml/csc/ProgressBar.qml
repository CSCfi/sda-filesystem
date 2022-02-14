import QtQuick 2.13
import QtQuick.Controls 2.13
import csc 1.0 as CSC

ProgressBar {
    id: control
    padding: 2

    background: Rectangle {
        implicitHeight: 12
        color: CSC.Style.lightGrey
        radius: 5
    }

    contentItem: Item {
        Rectangle {
            width: control.visualPosition * parent.width
            height: parent.height
            radius: (control.height - 2 * control.padding) / 2
            color: CSC.Style.turquiose
        }
    }
}