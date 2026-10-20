package storage

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"time"

	"github.com/uptrace/bun"
)

var ErrPoolFull=errors.New("storage pool full")
type Placement struct{ ID,Type,Path string; Available int64 }

func AvailableOnNode(ctx context.Context,db *bun.DB,driver StorageDriver,nodeID string)(int64,error){
 var n Placement;var capacity int64;var enabled bool
 err:=db.QueryRowContext(ctx,`SELECT id,type,COALESCE(path,''),capacity_bytes,enabled FROM storage_nodes WHERE id=?`,nodeID).Scan(&n.ID,&n.Type,&n.Path,&capacity,&enabled)
 if err!=nil{return 0,err};if !enabled{return 0,ErrPoolFull}
 used,err:=NodeUsedBytes(ctx,db.DB,nodeID)
 if err!=nil{return 0,err}
 var free int64
 if capacity <= 0 {
  free = 1000 * 1024 * 1024 * 1024 - used
 } else {
  free = capacity - capacity/20 - used
  if capacity%20 != 0 {
   free--
  }
 }
 if n.Type=="LOCAL" && driver!=nil {
  total,physicalFree,err:=driver.StatLocal(ctx,n.Path)
  if err!=nil{return 0,err}
  if total>0 {
   reserve:=total/20
   if total%20!=0{reserve++}
   if physicalFree-reserve<free{free=physicalFree-reserve}
  }
 }
 reserved:=GetNodeReserved(nodeID)
 free -= reserved
 if free<0{free=0}
 return free,nil
}

func SelectPlacement(ctx context.Context,db *bun.DB,driver StorageDriver,size int64)(Placement,error){
 rows,err:=db.QueryContext(ctx,`SELECT id,type,COALESCE(path,'') FROM storage_nodes WHERE enabled=1 ORDER BY created_at,id`);if err!=nil{return Placement{},err}
 var nodes []Placement
 for rows.Next(){var n Placement;if err=rows.Scan(&n.ID,&n.Type,&n.Path);err!=nil{break};nodes=append(nodes,n)}
 if err==nil{err=rows.Err()};_ =rows.Close();if err!=nil{return Placement{},err}
 var strategy string;_ =db.QueryRowContext(ctx,`SELECT value FROM settings WHERE key='placement_strategy'`).Scan(&strategy)
	var selected []Placement
	for _, n := range nodes {
		n.Available, err = AvailableOnNode(ctx, db, driver, n.ID)
		if err != nil {
			if n.Type == "S3" {
				continue
			}
			return Placement{}, err
		}
		if n.Available > 0 && (size <= 0 || n.Available >= size) {
			selected = append(selected, n)
		}
	}
	if len(selected) == 0 {
		return Placement{}, ErrPoolFull
	}
	switch strategy {
	case "MOST_FREE_SPACE":
		best := 0
		for i := 1; i < len(selected); i++ {
			if selected[i].Available > selected[best].Available {
				best = i
			}
		}
		return selected[best], nil
	case "ROUND_ROBIN":
		return selected[int(time.Now().UnixNano()%int64(len(selected)))], nil
	default:
		return selected[0], nil
	}
}

// ReadBlob retains the original local storage path for legacy records with no storage_node_id.
func ReadBlob(ctx context.Context,db *bun.DB,driver StorageDriver,table,key string)(io.ReadCloser,error){
 if table!="my_storage_items"&&table!="organization_items"{return nil,errors.New("invalid blob table")}
 var nodeID,path sql.NullString
 err:=db.QueryRowContext(ctx,`SELECT i.storage_node_id,sn.path FROM `+table+` i LEFT JOIN storage_nodes sn ON sn.id=i.storage_node_id WHERE i.disk_key=? ORDER BY i.storage_node_id IS NULL LIMIT 1`,key).Scan(&nodeID,&path)
 if err!=nil&& !errors.Is(err,sql.ErrNoRows){return nil,err}
 if nodeID.Valid {var kind string;if err=db.QueryRowContext(ctx,`SELECT type FROM storage_nodes WHERE id=?`,nodeID.String).Scan(&kind);err!=nil{return nil,err};if kind=="S3" {cloud,ok:=driver.(CloudStorageDriver);if !ok{return nil,errors.New("cloud driver unavailable")};return cloud.ReadObject(ctx,nodeID.String,key)}}
 return driver.ReadLocal(ctx,path.String,key)
}

func DeleteBlob(ctx context.Context,db *bun.DB,driver StorageDriver,nodeID,nodePath,key string)error{
 if nodeID!=""{var kind string;if err:=db.QueryRowContext(ctx,`SELECT type FROM storage_nodes WHERE id=?`,nodeID).Scan(&kind);err!=nil{return err};if kind=="S3"{cloud,ok:=driver.(CloudStorageDriver);if !ok{return errors.New("cloud driver unavailable")};return cloud.DeleteObject(ctx,nodeID,key)}}
 return driver.DeleteLocal(ctx,nodePath,key)
}
