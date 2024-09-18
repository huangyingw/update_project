package utils

import (
    "bufio"
    "os/exec"
    "strings"
)

func IsRemoteMounted(path string) (bool, error) {
    cmd := exec.Command("df", "-P", path)
    output, err := cmd.Output()
    if err != nil {
        return false, err
    }

    scanner := bufio.NewScanner(strings.NewReader(string(output)))
    var lastLine string
    for scanner.Scan() {
        lastLine = scanner.Text()
    }

    fields := strings.Fields(lastLine)
    if len(fields) > 0 && strings.Contains(fields[0], ":") {
        return true, nil
    }
    return false, nil
}
