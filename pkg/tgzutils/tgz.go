package tgzutils

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Uncompress decompresses a .tgz file into a directory.
func Uncompress(tgz, dst string) error {
	fp, err := os.Open(tgz)
	if err != nil {
		return fmt.Errorf("unable to open tgz file: %v", err)
	}
	defer fp.Close()

	gzreader, err := gzip.NewReader(fp)
	if err != nil {
		return fmt.Errorf("unable to create gzip reader: %v", err)
	}

	tarreader := tar.NewReader(gzreader)
	for {
		header, err := tarreader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("unable to read tar header: %v", err)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			mode := os.FileMode(header.Mode)
			dst := filepath.Join(dst, header.Name)
			if err := os.Mkdir(dst, mode); err != nil {
				return fmt.Errorf("unable to create directory: %v", err)
			}
		case tar.TypeReg:
			mode := os.FileMode(header.Mode)
			dst := filepath.Join(dst, header.Name)
			opts := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
			outfp, err := os.OpenFile(dst, opts, mode)
			if err != nil {
				return fmt.Errorf("unable to create file: %v", err)
			}
			if _, err := io.Copy(outfp, tarreader); err != nil {
				return fmt.Errorf("unable to write file: %v", err)
			}
			outfp.Close()
			if err := os.Chmod(dst, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("unable to chmod file: %v", err)
			}
		default:
			return fmt.Errorf("unknown type: %v", header.Typeflag)
		}
	}
	return nil
}
