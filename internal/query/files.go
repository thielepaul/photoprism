package query

import (
	"fmt"
	"path"
	"strings"

	"github.com/photoprism/photoprism/internal/entity"
)

// FilesByPath returns a slice of files in a given originals folder.
func FilesByPath(limit, offset int, rootName, pathName string) (files entity.Files, err error) {
	if strings.HasPrefix(pathName, "/") {
		pathName = pathName[1:]
	}

	err = Db().
		Table("files").Select("files.*").
		Joins("JOIN photos ON photos.id = files.photo_id AND photos.deleted_at IS NULL").
		Where("files.file_missing = 0").
		Where("files.file_root = ? AND photos.photo_path = ?", rootName, pathName).
		Order("files.file_name").
		Limit(limit).Offset(offset).
		Find(&files).Error

	return files, err
}

// Files returns not-missing and not-deleted file entities in the range of limit and offset sorted by id.
func Files(limit, offset int, pathName string, includeMissing bool) (files entity.Files, err error) {
	if strings.HasPrefix(pathName, "/") {
		pathName = pathName[1:]
	}

	stmt := Db()

	if !includeMissing {
		stmt = stmt.Where("file_missing = 0")
	}

	if pathName != "" {
		stmt = stmt.Where("files.file_name LIKE ?", pathName+"/%")
	}

	err = stmt.Order("id").Limit(limit).Offset(offset).Find(&files).Error

	return files, err
}

// FilesByUID
func FilesByUID(u []string, limit int, offset int) (files entity.Files, err error) {
	if err := Db().Where("(photo_uid IN (?) AND file_primary = 1) OR file_uid IN (?)", u, u).Preload("Photo").Limit(limit).Offset(offset).Find(&files).Error; err != nil {
		return files, err
	}

	return files, nil
}

// FileByPhotoUID
func FileByPhotoUID(u string) (file entity.File, err error) {
	if err := Db().Where("photo_uid = ? AND file_primary = 1", u).Preload("Photo").First(&file).Error; err != nil {
		return file, err
	}

	return file, nil
}

// VideoByPhotoUID
func VideoByPhotoUID(u string) (file entity.File, err error) {
	if err := Db().Where("photo_uid = ? AND file_video = 1", u).Preload("Photo").First(&file).Error; err != nil {
		return file, err
	}

	return file, nil
}

// FileByUID returns the file entity for a given UID.
func FileByUID(uid string) (file entity.File, err error) {
	if err := Db().Where("file_uid = ?", uid).Preload("Photo").First(&file).Error; err != nil {
		return file, err
	}

	return file, nil
}

// FileByHash finds a file with a given hash string.
func FileByHash(fileHash string) (file entity.File, err error) {
	if err := Db().Where("file_hash = ?", fileHash).Preload("Photo").First(&file).Error; err != nil {
		return file, err
	}

	return file, nil
}

// RenameFile renames an indexed file.
func RenameFile(srcRoot, srcName, destRoot, destName string) error {
	if srcRoot == "" || srcName == "" || destRoot == "" || destName == "" {
		return fmt.Errorf("can't rename %s/%s to %s/%s", srcRoot, srcName, destRoot, destName)
	}

	return Db().Exec("UPDATE files SET file_root = ?, file_name = ?, file_missing = 0, deleted_at = NULL WHERE file_root = ? AND file_name = ?", destRoot, destName, srcRoot, srcName).Error
}

// SetPhotoPrimary sets a new primary image file for a photo.
func SetPhotoPrimary(photoUID, fileUID string) error {
	if photoUID == "" {
		return fmt.Errorf("photo uid is missing")
	}

	var files []string

	if fileUID != "" {
		// Do nothing.
	} else if err := Db().Model(entity.File{}).Where("photo_uid = ? AND file_missing = 0 AND file_type = 'jpg'", photoUID).Order("file_width DESC").Limit(1).Pluck("file_uid", &files).Error; err != nil {
		return err
	} else if len(files) == 0 {
		return fmt.Errorf("can't find primary file for %s", photoUID)
	} else {
		fileUID = files[0]
	}

	if fileUID == "" {
		return fmt.Errorf("file uid is missing")
	}

	Db().Model(entity.File{}).Where("photo_uid = ? AND file_uid <> ?", photoUID, fileUID).Update("file_primary", 0)
	return Db().Model(entity.File{}).Where("photo_uid = ? AND file_uid = ?", photoUID, fileUID).Update("file_primary", 1).Error
}

// SetFileError updates the file error column.
func SetFileError(fileUID, errorString string) {
	if err := Db().Model(entity.File{}).Where("file_uid = ?", fileUID).Update("file_error", errorString).Error; err != nil {
		log.Errorf("query: %s", err.Error())
	}
}

type FileMap map[string]int64

// IndexedFiles returns a map of already indexed files with their mod time unix timestamp as value.
func IndexedFiles() (result FileMap, err error) {
	result = make(FileMap)

	type File struct {
		FileRoot string
		FileName string
		ModTime  int64
	}

	// Query known duplicates.
	var duplicates []File

	if err := UnscopedDb().Raw("SELECT file_root, file_name, mod_time FROM duplicates").Scan(&duplicates).Error; err != nil {
		return result, err
	}

	for _, row := range duplicates {
		result[path.Join(row.FileRoot, row.FileName)] = row.ModTime
	}

	// Query indexed files.
	var files []File

	if err := UnscopedDb().Raw("SELECT file_root, file_name, mod_time FROM files WHERE file_missing = 0 AND deleted_at IS NULL").Scan(&files).Error; err != nil {
		return result, err
	}

	for _, row := range files {
		result[path.Join(row.FileRoot, row.FileName)] = row.ModTime
	}

	return result, err
}

type HashMap map[string]bool

// FileHashes returns a map of all known file hashes.
func FileHashes() (result HashMap, err error) {
	result = make(HashMap)

	var hashes []string

	if err := UnscopedDb().Raw("SELECT file_hash FROM files WHERE file_missing = 0 AND deleted_at IS NULL").Pluck("file_hash", &hashes).Error; err != nil {
		return result, err
	}

	for _, hash := range hashes {
		result[hash] = true
	}

	return result, err
}
