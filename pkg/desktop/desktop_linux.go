//go:build linux

package desktop

import (
	"fmt"
	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
	"os"
)

const trayPath = dbus.ObjectPath("/StatusNotifierItem")
const menuPath = dbus.ObjectPath("/MenuBar")
const trayInterface = "org.kde.StatusNotifierItem"
const menuInterface = "com.canonical.dbusmenu"

type trayLinux struct{ actions Actions }

func (t *trayLinux) Activate(x, y int32) *dbus.Error                    { t.actions.Open(); return nil }
func (t *trayLinux) SecondaryActivate(x, y int32) *dbus.Error           { t.actions.Open(); return nil }
func (t *trayLinux) ContextMenu(x, y int32) *dbus.Error                 { t.actions.Open(); return nil }
func (t *trayLinux) Scroll(delta int32, orientation string) *dbus.Error { return nil }

type menuNode struct {
	ID         int32
	Properties map[string]dbus.Variant
	Children   []dbus.Variant
}
type linuxMenu struct {
	labels  []string
	actions []func()
}

func (m *linuxMenu) node(id int32) menuNode {
	node := menuNode{ID: id, Properties: map[string]dbus.Variant{}, Children: []dbus.Variant{}}
	if id == 0 {
		node.Properties["children-display"] = dbus.MakeVariant("submenu")
		for i := range m.labels {
			node.Children = append(node.Children, dbus.MakeVariant(m.node(int32(i+1))))
		}
	} else if id > 0 && int(id) <= len(m.labels) {
		node.Properties = map[string]dbus.Variant{"label": dbus.MakeVariant(m.labels[id-1]), "enabled": dbus.MakeVariant(true), "visible": dbus.MakeVariant(true)}
	}
	return node
}
func (m *linuxMenu) GetLayout(parent, depth int32, names []string) (uint32, menuNode, *dbus.Error) {
	return 1, m.node(parent), nil
}

type menuProperties struct {
	ID         int32
	Properties map[string]dbus.Variant
}

func (m *linuxMenu) GetGroupProperties(ids []int32, names []string) ([]menuProperties, *dbus.Error) {
	if len(ids) == 0 {
		for i := range m.labels {
			ids = append(ids, int32(i+1))
		}
	}
	result := []menuProperties{}
	for _, id := range ids {
		result = append(result, menuProperties{id, m.node(id).Properties})
	}
	return result, nil
}
func (m *linuxMenu) GetProperty(id int32, name string) (dbus.Variant, *dbus.Error) {
	v, ok := m.node(id).Properties[name]
	if !ok {
		return dbus.MakeVariant(""), dbus.NewError("com.canonical.dbusmenu.Error.PropertyNotFound", nil)
	}
	return v, nil
}
func (m *linuxMenu) Event(id int32, event string, data dbus.Variant, timestamp uint32) *dbus.Error {
	if event == "clicked" && id > 0 && int(id) <= len(m.actions) {
		m.actions[id-1]()
	}
	return nil
}
func (m *linuxMenu) AboutToShow(id int32) (bool, *dbus.Error) { return false, nil }
func Install(a Actions) (func(), bool, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return func() {}, false, nil
	}
	cleanup := func() { _ = conn.Close() }
	name := fmt.Sprintf("org.kde.StatusNotifierItem-%d-1", os.Getpid())
	if _, err = conn.RequestName(name, dbus.NameFlagDoNotQueue); err != nil {
		cleanup()
		return nil, false, err
	}
	tray := &trayLinux{a}
	if err = conn.Export(tray, trayPath, trayInterface); err != nil {
		cleanup()
		return nil, false, err
	}
	menu := &linuxMenu{[]string{"Open Dym Bridge", "Sites…", "Connect / Reconnect", "Disconnect", "Quit Dym Bridge"}, []func(){a.Open, a.Sites, a.Connect, a.Disconnect, a.Quit}}
	if err = conn.Export(menu, menuPath, menuInterface); err != nil {
		cleanup()
		return nil, false, err
	}
	_, err = prop.Export(conn, trayPath, prop.Map{trayInterface: {
		"Category": {Value: "ApplicationStatus"}, "Id": {Value: "dym-bridge"}, "Title": {Value: "Dym Bridge"}, "Status": {Value: "Active"}, "IconName": {Value: "network-server"}, "ItemIsMenu": {Value: false}, "Menu": {Value: menuPath},
	}})
	if err != nil {
		cleanup()
		return nil, false, err
	}
	_, err = prop.Export(conn, menuPath, prop.Map{menuInterface: {"Version": {Value: uint32(3)}, "TextDirection": {Value: "ltr"}, "Status": {Value: "normal"}, "IconThemePath": {Value: []string{}}}})
	if err != nil {
		cleanup()
		return nil, false, err
	}
	call := conn.Object("org.kde.StatusNotifierWatcher", "/StatusNotifierWatcher").Call("org.kde.StatusNotifierWatcher.RegisterStatusNotifierItem", 0, name)
	if call.Err != nil {
		cleanup()
		return func() {}, false, nil
	}
	return cleanup, true, nil
}
