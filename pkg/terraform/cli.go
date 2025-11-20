package terraform

import (
	"fmt"
	"os"
	"os/exec"
)

// TerraformExecutor は terraform CLI をラップし、init / import などの操作を提供するインターフェース。
// F-06 詳細設計の TerraformExecutor に対応する。
type TerraformExecutor interface {
	Init(tfDir string) error
	Import(tfDir string, address string, id string) error
}

// DefaultTerraformExecutor はローカルの terraform バイナリを利用するデフォルト実装。
type DefaultTerraformExecutor struct {
	// TerraformBin は terraform バイナリ名またはパス。
	// 空の場合は "terraform" を利用する。
	TerraformBin string
}

// NewDefaultTerraformExecutor は DefaultTerraformExecutor を生成する。
func NewDefaultTerraformExecutor() *DefaultTerraformExecutor {
	return &DefaultTerraformExecutor{
		TerraformBin: "terraform",
	}
}

// Init は指定された tfDir をワーキングディレクトリとして terraform init を実行する。
func (e *DefaultTerraformExecutor) Init(tfDir string) error {
	bin := e.TerraformBin
	if bin == "" {
		bin = "terraform"
	}

	cmd := exec.Command(bin, "init", "-input=false")
	cmd.Dir = tfDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("terraform init failed: %w", err)
	}
	return nil
}

// Import は指定された tfDir をワーキングディレクトリとして terraform import を実行する。
func (e *DefaultTerraformExecutor) Import(tfDir string, address string, id string) error {
	bin := e.TerraformBin
	if bin == "" {
		bin = "terraform"
	}

	cmd := exec.Command(bin, "import", address, id)
	cmd.Dir = tfDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("terraform import %s %s failed: %w", address, id, err)
	}
	return nil
}


