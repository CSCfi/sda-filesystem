import QtQuick 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls 2.13
import QtQuick.Controls.Material 2.12
import csc 1.0 as CSC

ApplicationWindow {
    visible: true
    title: "SD-Connect FUSE"
	width: 700 // TODO: modify
	height: 500 // TODO: modify

	property string username

	header: TabBar {
		Material.accent: CSC.Style.secondaryColor

        TabButton {
        	text: qsTr("Home")
			width: implicitWidth
    	}
		TabButton {
        	text: qsTr("Logs")
			width: implicitWidth
    	}
		TabButton {
        	text: qsTr("Statistics?")
			width: implicitWidth
    	}
    }

	Label {
		text: "<h2>Logged in as " + username + "</h2>"
		color: "black"
	}
			
	RowLayout {
		anchors.fill: parent

		ColumnLayout {
			Layout.fillHeight: true
			Layout.fillWidth: true
			Layout.alignment: Qt.AlignTop

			Frame {
				//Layout.fillWidth: true

				background: Rectangle {
					border.color: CSC.Style.secondaryColor
					border.width: 5
				}
			}
		}

		GroupBox { // groupbox?
			title: "Projects"
			Layout.fillHeight: true
			Layout.fillWidth: true

			ListView { // go struct

			}
		}
	}

	
}