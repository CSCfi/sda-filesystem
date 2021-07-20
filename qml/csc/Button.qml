Button {
    hoverEnabled: true
    padding: 15
    Material.foreground: loginButton.enabled ? "white" : CSC.Style.disabledForeground

    background: Rectangle {
        radius: 4
        color: loginButton.enabled ? (loginButton.pressed ? "#9BBCB7" : (loginButton.hovered ? "#61958D" : CSC.Style.primaryColor)) : CSC.Style.disabledBackground
    }
}