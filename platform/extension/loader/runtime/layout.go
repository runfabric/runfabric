package runtime

import "path/filepath"

func BuildDir(root string) string {
	return filepath.Join(root, ".runfabric", "build")
}

func ArtifactPath(root, function, buildKey string) string {
	return filepath.Join(BuildDir(root), function+"-"+buildKey+".zip")
}

func ManifestPath(root, stage string) string {
	return filepath.Join(root, ".runfabric", "artifacts-"+stage+".json")
}
