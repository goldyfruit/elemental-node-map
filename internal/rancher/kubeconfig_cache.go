package rancher

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const DefaultKubeconfigCacheTTL = 10 * time.Minute

func KubeconfigCacheKey(baseURL string, clusterID string) string {
	return baseURL + "|" + clusterID
}

func LoadCachedKubeconfig(key string, ttl time.Duration) ([]byte, time.Duration, bool, error) {
	path, err := kubeconfigCachePath(key)
	if err != nil {
		return nil, 0, false, err
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, false, nil
		}
		return nil, 0, false, err
	}
	age := time.Since(info.ModTime())
	if ttl > 0 && age > ttl {
		return nil, age, false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, false, err
	}
	return data, age, true, nil
}

func SaveCachedKubeconfig(key string, data []byte) error {
	path, err := kubeconfigCachePath(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func kubeconfigCachePath(key string) (string, error) {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	sum := sha256.Sum256([]byte(key))
	name := fmt.Sprintf("kubeconfig-%s.yaml", hex.EncodeToString(sum[:16]))
	return filepath.Join(base, "elemental-node-map", name), nil
}
