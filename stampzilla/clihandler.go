package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"github.com/sirupsen/logrus"
	"github.com/stampzilla/stampzilla-go/pkg/build"
	"github.com/stampzilla/stampzilla-go/stampzilla/installer"
	"github.com/stampzilla/stampzilla-go/stampzilla/installer/binary"
	"github.com/stampzilla/stampzilla-go/stampzilla/runner"
	"github.com/urfave/cli"
)

type cliHandler struct {
}

func (t *cliHandler) UpdateConfig(c *cli.Context) error {
	requireRoot()

	oldConfig := &installer.Config{}
	oldConfig.ReadConfigFromFile("/etc/stampzilla/nodes.conf")

	newConfig := &installer.Config{}
	newConfig.GenerateDefault()
	for _, n := range newConfig.Daemons {
		if old := oldConfig.GetConfigForNode(n.Name); old != nil {
			n.Autostart = old.Autostart
		}
	}
	return newConfig.SaveToFile("/etc/stampzilla/nodes.conf")
}

func (t *cliHandler) Install(c *cli.Context) error {
	i, err := installer.New(installer.Binaries)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err,
		}).Error("Failed to create installer")
		return nil
	}

	return t.runInstaller(c, i)
}

func (t *cliHandler) Build(c *cli.Context) error {
	i, err := installer.New(installer.SourceCode)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err,
		}).Error("Failed to create installer")
		return nil
	}

	return t.runInstaller(c, i)
}

func (t *cliHandler) List(c *cli.Context) error {
	client := github.NewClient(nil)
	ctx := context.Background()
	releases, _, err := client.Repositories.ListReleases(ctx, "stampzilla", "stampzilla-go", &github.ListOptions{})

	if err != nil {
		return err
	}

	for _, v := range releases {
		fmt.Println(*v.TagName)
	}
	return nil
}

func (t *cliHandler) runInstaller(c *cli.Context, i installer.Installer) error {
	requireRoot()

	err := installer.Prepare()
	if err != nil {
		return fmt.Errorf("Failed to run common prepare: %s", err)
	}

	err = i.Prepare()
	if err != nil {
		return fmt.Errorf("Failed to run installer prepare: %s", err)
	}

	if c.Bool("u") {
		err = i.Update(c.Args()...)
	} else {
		err = i.Install(c.Args()...)
	}
	if err != nil {
		return err
	}

	if c.GlobalBool("systemd") || c.GlobalBoolT("systemd") {
		r := &runner.Systemd{}
		defer r.Close()
		if len(c.Args()) > 0 {
			for _, node := range c.Args() {
				err := r.GenerateUnit(node)
				if err != nil {
					return err
				}
			}
			return nil
		}

		// generate for all nodes in binary dir
		nodes, err := ioutil.ReadDir("/home/stampzilla/go/bin")
		if err != nil {
			return err
		}
		for _, node := range nodes {
			if node.IsDir() {
				continue
			}
			if node.Name() == "stampzilla" { // skip stampzilla cli if it exists
				continue
			}
			if !strings.HasPrefix(node.Name(), "stampzilla-") {
				continue
			}
			err := r.GenerateUnit(node.Name())
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (t *cliHandler) Start(c *cli.Context) error {
	requireRoot()
	r := getRunner(c)
	defer r.Close()
	return r.Start(c.Args()...)
}

func (t *cliHandler) Stop(c *cli.Context) error {
	requireRoot()
	r := getRunner(c)
	defer r.Close()
	return r.Stop(c.Args()...)
}

func (t *cliHandler) Restart(c *cli.Context) error {
	requireRoot()
	r := getRunner(c)
	defer r.Close()

	return r.Restart(c.Args()...)
}
func (t *cliHandler) Status(c *cli.Context) error {
	requireRoot()
	r := getRunner(c)
	defer r.Close()
	return r.Status()
}
func (t *cliHandler) Disable(c *cli.Context) error {
	requireRoot()
	r := &runner.Systemd{}
	defer r.Close()
	return r.Disable(c.Args()...)
}

func (t *cliHandler) Debug(c *cli.Context) error {
	requireRoot()

	what := c.Args().First()
	shbin, err := exec.LookPath("sh")
	if err != nil {
		return fmt.Errorf("LookPath Error: %s", err)
	}

	confDir := "/etc/stampzilla/nodes/" + what
	installer.CreateDirAsUser(confDir, "stampzilla")
	chdircmd := " cd " + confDir + "; "

	toRun := chdircmd + "$GOPATH/bin/" + runner.GetProcessName(what)
	cmd := exec.Command("sudo", "-E", "-u", "stampzilla", "-H", shbin, "-c", toRun)
	cmd.Env = []string{
		"GOPATH=/home/stampzilla/go",
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (t *cliHandler) Log(c *cli.Context) error {
	follow := c.Bool("f")

	var cmd *exec.Cmd
	if c.GlobalBool("systemd") || c.GlobalBoolT("systemd") {
		cmd = exec.Command("journalctl", "-u", runner.GetProcessName(c.Args().First()))
		if follow {
			cmd = exec.Command("journalctl", "-f", "-u", runner.GetProcessName(c.Args().First()))
		}
	} else {
		cmd = exec.Command("less", "-R", "/var/log/stampzilla/"+runner.GetProcessName(c.Args().First())+".log")
		if follow {
			cmd = exec.Command("tail", "-f", "/var/log/stampzilla/"+runner.GetProcessName(c.Args().First())+".log")
		}
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (t *cliHandler) SelfUpdate(c *cli.Context) error {
	releases := binary.GetReleases()
	if releases == nil {
		return fmt.Errorf("error fetching releases from github.com")
	}

	if len(releases) == 0 {
		return fmt.Errorf("found 0 releases on github.com")
	}

	version := releases[0].GetName()

	if version == build.Version {
		logrus.Info("Found no new version")
		return nil
	}

	logrus.Infof("Found new version %s (old: %s)\n", version, build.Version)

	asset := fmt.Sprintf("stampzilla-%s-%s", runtime.GOOS, runtime.GOARCH)
	u := ""
	for _, a := range releases[0].Assets {
		if a.GetName() == asset {
			u = a.GetBrowserDownloadURL()
		}
	}

	if u == "" {
		return fmt.Errorf("Found no asset to download named %s", asset)
	}

	binPath, err := os.Executable()
	if err != nil {
		return err
	}

	client := http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	err = os.Rename(binPath, binPath+".backup")
	if err != nil {
		return fmt.Errorf(`%v
Possible solutions:
1: sudo stampzilla self-update
2: put the binary in ~/bin and make sure you have full permissions and the directory is in your PATH`, err)
	}

	file, err := os.OpenFile(binPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0777) // #nosec
	if err != nil {
		return err
	}

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		err = os.Rename(binPath+".backup", binPath)
		if err != nil {
			return fmt.Errorf("Failed to restore backup: %s", err.Error())
		}
		return err
	}

	err = os.Remove(binPath + ".backup")
	if err != nil {
		return err
	}

	logrus.Infof("Update to version %s successful", version)
	return nil
}

func getRunner(c *cli.Context) runner.Runner {
	if c.GlobalBool("systemd") || c.GlobalBoolT("systemd") {
		logrus.Debug("systemd enabled")
		return &runner.Systemd{}
	}
	logrus.Debug("systemd disabled")
	return &runner.Bare{}

}

func requireRoot() { // {{{
	if os.Getuid() != 0 {
		fmt.Println("You must be root to run this")
		os.Exit(1)
	}
} // }}}
