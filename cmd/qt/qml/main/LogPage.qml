import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls.Material 2.12
import csc 1.0 as CSC

Page {
    ListView {
        anchors.fill: parent
        boundsBehavior: Flickable.StopAtBounds
        verticalLayoutDirection: ListView.BottomToTop

        model: LogModel
        delegate: Text { text: level + " " + timestamp + " " + message; font: QmlBridge.fixedFont }
    }
}