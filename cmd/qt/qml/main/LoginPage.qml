import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls.Material 2.12
import QtQml 2.13
import csc 1.2 as CSC

Page {
    id: page
    height: content.height + 2 * CSC.Style.padding
    implicitHeight: height
    implicitWidth: content.width + 2 * CSC.Style.padding
    
    Material.accent: CSC.Style.primaryColor
    Material.foreground: CSC.Style.grey

    Component.onCompleted: implicitHeight = content.height + 2 * CSC.Style.padding

    Keys.onReturnPressed: loginButton.clicked() // Enter key
    Keys.onEnterPressed: loginButton.clicked()  // Numpad enter key

    Connections {
        target: QmlBridge
        onLoginFail: {
            passwordField.errorVisible = true
            retryLogin()
        }
        onPopupError: if (!QmlBridge.loggedIn) {
            retryLogin()
        }
        onLoggedInChanged: if (QmlBridge.loggedIn) {
            loginButton.loading = false
        }
    }

    function retryLogin() {
        loginButton.loading = false
        usernameField.enabled = true
        passwordField.enabled = true

        if (usernameField.text != "") {
            passwordField.focus = true
            passwordField.selectAll()
        }
    }

    Column {
        id: content
        spacing: CSC.Style.padding
        height: childrenRect.height + topPadding
        width: childrenRect.width + leftPadding
        topPadding: 2 * CSC.Style.padding
        leftPadding: 2 * CSC.Style.padding

        Label {
            text: "<h1>Log in to Data Gateway</h1>"
            color: CSC.Style.primaryColor
            maximumLineCount: 1
        }

        Label {
            text: "Data Gateway gives you secure access to your data."
            lineHeight: 1.2
            font.pixelSize: 14
            maximumLineCount: 1
        }

        Label {
            text: "Please log in with your CSC credentials."
            topPadding: 10
            font.pixelSize: 13
            maximumLineCount: 1
        }

        CSC.TextField {
            id: usernameField
            focus: true
            titleText: "Username"
            width: 400
        }

        CSC.TextField {
            id: passwordField
            titleText: "Password"
            errorText: "Please enter valid password"
            echoMode: TextInput.Password
            activeFocusOnTab: true
            width: 400
        }

        CSC.Button {
            id: loginButton
            text: "Login"

            onClicked: {
                popup.close()
                usernameField.enabled = false
                passwordField.enabled = false
                passwordField.errorVisible = false
                loading = true
                QmlBridge.login(usernameField.text, passwordField.text)
            }
        }
    }
}