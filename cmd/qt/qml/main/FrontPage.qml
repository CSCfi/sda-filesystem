import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls.Material 2.12
import Qt.labs.qmlmodels 1.0
import QtQuick.Dialogs 1.3
import csc 1.0 as CSC

Page {
    id: page
    padding: 2 * CSC.Style.padding

    FileDialog {
        id: dialogCreate
        title: "Choose or create a folder"
        folder: shortcuts.home
        selectExisting: false
        selectFolder: true
        onAccepted: {
            var mountError = QmlBridge.changeMountPoint(dialogCreate.fileUrl)
            if (mountError) {
                popup.errorMessage = mountError
                popup.open()
            }
        }
    }

    Connections {
        target: QmlBridge
        onPanic: {
            popupPanic.closePolicy = Popup.NoAutoClose // User must choose ignore or quit
            popupPanic.open()
        }
    }

    /*CSC.Popup {
		id: popupPanic
		errorMessage: "How can this be! Filesystem failed to load correctly.\nSave logs to find out why this happened and either quit the application or continue at your own peril..."

		Connections {
            target: QmlBridge
            onPanic: {
				popupPanic.closePolicy = Popup.NoAutoClose // User must choose ignore or quit
				popupPanic.open()
			}
        }
		
		Connections {
			target: dialogSave
			onReady: {
				if (ignoreButton.checked) {
					popupPanic.close()
				} else if (quitButton.checked) {
					close()
				}
			}
		}

		ColumnLayout {
			width: parent.width

			CheckBox {
				id: logCheck
				checked: true
				text: "Yes, save logs to file"

				Material.accent: CSC.Style.primaryColor
			}

			Row {
				spacing: CSC.Style.padding
				Layout.alignment: Qt.AlignRight

				CSC.Button {
					id: ignoreButton
					text: "Ignore"
					outlined: true
					checkable: true

					onClicked: {
						if (logCheck.checked) {
							dialogSave.visible = true
						} else {
							popupPanic.close()
						}
					}
				}

				CSC.Button {
					id: quitButton
					text: "Quit"
					checkable: true
					
					onClicked: {
						if (logCheck.checked) {
							dialogSave.visible = true
						} else {
							close()
						}
					}
				}
			}
		}
	}*/

    ColumnLayout {
        spacing: 2 * CSC.Style.padding

        ColumnLayout {
            spacing: CSC.Style.padding

            Text {
                text: "<h1>1. Choose directory</h1>"
                color: CSC.Style.grey
                maximumLineCount: 1

                // This version of Qt has a bug where the default font does not add a space after punctuation
                Component.onCompleted: {
                    if (Qt.platform.os == "osx") {
                        font.family = "Arial"
                    }  
                }
            }

            Text {
                text: "Choose in which local directory your data will be available"
                color: CSC.Style.grey
            }

            Row {
                spacing: CSC.Style.padding

                Rectangle {
                    radius: 5
                    border.width: 1
                    border.color: CSC.Style.lineGray
                    width: 300
                    height: childrenRect.height
                    anchors.verticalCenter: changeButton.verticalCenter

                    Flickable {
                        clip: true
                        width: parent.width
                        height: mountText.height
                        contentWidth: mountText.width
                        boundsBehavior: Flickable.StopAtBounds

                        ScrollBar.horizontal: ScrollBar { interactive: false }
                        
                        Text {
                            id: mountText
                            text: QmlBridge.mountPoint
                            font.pointSize: 15
                            verticalAlignment: Text.AlignVCenter
                            maximumLineCount: 1
                            padding: 10
                        }
                    }
                }

                CSC.Button {
                    id: changeButton
                    text: "Change"
                    outlined: true

                    onClicked: { popup.close(); fileDialog.visible = true }
                }
            }
        }

        ColumnLayout {
            spacing: CSC.Style.padding

            Text {
                text: "<h1>2. Mount directory</h1>"
                color: CSC.Style.grey

                Component.onCompleted: {
                    if (Qt.platform.os == "osx") {
                        font.family = "Arial"
                    }  
                }
            }

            TextMetrics {
                id: textMetrics100
                text: "100 %"
            }

            /*TableView {
                id: projectView
                interactive: false
                boundsBehavior: Flickable.StopAtBounds
                Layout.minimumHeight: contentHeight
                Layout.fillHeight: true
                Layout.fillWidth: true

                property real rowHeight: 50
                property real viewPadding: 10

                Material.accent: CSC.Style.altGreen

                property bool ready: false
                property real numColumnMinWidth: textMetrics100.width + 2 * viewPadding
                property real nameColumnMaxWidth

                rowHeightProvider: function (row) { return rowHeight }
                columnWidthProvider: function (column) { return column == 0 ? -1 : 0 } // Some shenanigans so that we can figure out nameColumnMaxWidth

                model: ProjectModel
                delegate: chooser

                Component.onCompleted: {
                    forceLayout()
                    nameColumnMaxWidth = projectView.contentWidth
                    columnWidthProvider = function (column) { 
                        if (column == 0) {
                            return Math.min(nameColumnMaxWidth, projectView.width - numColumnMinWidth)
                        } else {
                            return Math.max(numColumnMinWidth, projectView.width - nameColumnMaxWidth)
                        }
                    }
                    ready = true
                }

                onWidthChanged: {
                    if (ready) { // <- Otherwise error
                        forceLayout()
                    }
                }

                ScrollBar.vertical: ScrollBar { }
            }

            DelegateChooser {
                id: chooser

                DelegateChoice {
                    column: 0
                    delegate: Rectangle {
                        implicitHeight: projectView.viewPadding
                        implicitWidth: projectNameText.width
                        color: "transparent"
                        border.color: CSC.Style.lightGrey
                        border.width: 1

                        Flickable {
                            clip: true
                            contentWidth: projectNameText.width
                            interactive: contentWidth > width
                            boundsBehavior: Flickable.StopAtBounds
                            anchors.fill: parent

                            ScrollIndicator.horizontal: ScrollIndicator { }
                            
                            Text {
                                id: projectNameText
                                text: projectName
                                font.pointSize: 15
                                font.weight: Font.Medium
                                verticalAlignment: Text.AlignVCenter
                                maximumLineCount: 1
                                padding: projectView.viewPadding
                                color: CSC.Style.grey
                                height: parent.height
                            }
                        }

                    }
                }

                DelegateChoice {
                    column: 1
                    delegate: Rectangle {
                        implicitHeight: projectView.rowHeight
                        color: "transparent"
                        border.color: CSC.Style.lightGrey
                        border.width: 1

                        RowLayout {
                            anchors.fill: parent
                            anchors.rightMargin: projectView.viewPadding
                            anchors.leftMargin: projectView.viewPadding
                            property real value: (allContainers == -1) ? 0 : (allContainers == 0) ? 1 : loadedContainers / allContainers

                            CSC.ProgressBar {
                                id: progressbar
                                value: parent.value
                                Layout.fillWidth: true
                            }

                            Text {
                                id: percentValue
                                text: Math.round(parent.value * 100) + " %"
                                maximumLineCount: 1
                                color: CSC.Style.grey
                                Layout.minimumWidth: textMetrics100.width
                            }

                            onWidthChanged: {
                                progressbar.visible = (width > 100)
                            }
                        }

                    }
                }
            }*/
        }
    }
}