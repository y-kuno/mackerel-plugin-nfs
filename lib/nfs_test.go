package mpnfs

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var str = `
device rootfs mounted on / with fstype rootfs
device proc mounted on /proc with fstype proc
device sysfs mounted on /sys with fstype sysfs
device devtmpfs mounted on /dev with fstype devtmpfs
device devpts mounted on /dev/pts with fstype devpts
device tmpfs mounted on /dev/shm with fstype tmpfs
device /dev/vda1 mounted on / with fstype ext4
device /proc/bus/usb mounted on /proc/bus/usb with fstype usbfs
device none mounted on /proc/sys/fs/binfmt_misc with fstype binfmt_misc
device sunrpc mounted on /var/lib/nfs/rpc_pipefs with fstype rpc_pipefs
device 0.0.0.0:/data/ mounted on /mnt with fstype nfs4 statvers=1.1
	opts:	rw,vers=4,rsize=1048576,wsize=1048576,namlen=255,acregmin=3,acregmax=60,acdirmin=30,acdirmax=60,hard,proto=tcp,port=0,timeo=600,retrans=2,sec=sys,clientaddr=0.0.0.0,minorversion=0,local_lock=none
	age:	1969800
	caps:	caps=0xfff7,wtmult=512,dtsize=32768,bsize=0,namlen=255
	nfsv4:	bm0=0xfdffafff,bm1=0xf9be3e,acl=0x0
	sec:	flavor=1,pseudoflavor=1
	events:	1228468 14061231 16253 66023 700996 13686 14862276 16991407 0 10404 11302229 1197709 1401992 4487 4488 4488 0 17701 0 0 16991407 1 0 0 0 0 0
	bytes:	49181634820 46217103388 0 0 34775617317 46248035739 8494353 11302229
	RPC iostats version: 1.0  p/v: 100003/4 (nfs)
	xprt:	tcp 707 0 1 0 28 2019539 2019539 0 2533424 0 259 756960 513900
	per-op statistics
	        NULL: 0 0 0 0 0 0 0 0
	        READ: 43026 43026 0 7228368 34778206720 1782 2197001 2206478
	       WRITE: 60192 60192 0 46259381156 7945344 4905359 1564953 6473964
	      COMMIT: 8991 8991 0 1366632 539460 8952 120129 129368
	        OPEN: 13756 13756 0 3445016 5309260 297 10093 10719
	OPEN_CONFIRM: 1896 1896 0 303360 128928 13 358 398
	 OPEN_NOATTR: 0 0 0 0 0 0 0 0
	OPEN_DOWNGRADE: 0 0 0 0 0 0 0 0
	       CLOSE: 13756 13756 0 2421056 1815792 230 2858 3364
	     SETATTR: 1 1 0 196 264 0 1 1
	      FSINFO: 1 1 0 144 108 0 0 0
	       RENEW: 0 0 0 0 0 0 0 0
	 SETCLIENTID: 0 0 0 0 0 0 0 0
	SETCLIENTID_CONFIRM: 0 0 0 0 0 0 0 0
	        LOCK: 0 0 0 0 0 0 0 0
	       LOCKT: 0 0 0 0 0 0 0 0
	       LOCKU: 0 0 0 0 0 0 0 0
	      ACCESS: 427753 427753 0 68384968 111215780 1477 67864 86237
	     GETATTR: 1228469 1228469 0 186582800 299746436 4844 193841 242274
	      LOOKUP: 17390 17390 0 3111992 2139560 171 4121 4670
	 LOOKUP_ROOT: 0 0 0 0 0 0 0 0
	      REMOVE: 1536 1536 0 247200 110572 7 9267 9305
	      RENAME: 4287 4287 0 908860 462996 36 6500 6616
	        LINK: 0 0 0 0 0 0 0 0
	     SYMLINK: 0 0 0 0 0 0 0 0
	      CREATE: 5455 5455 0 1135820 1767420 28 18146 18516
	    PATHCONF: 1 1 0 140 72 0 0 0
	      STATFS: 117476 117476 0 16916544 13627216 2452 46563 70896
	    READLINK: 0 0 0 0 0 0 0 0
	     READDIR: 35882 35882 0 6315208 79909924 130 8432 9320
	 SERVER_CAPS: 2 2 0 280 176 0 0 0
	 DELEGRETURN: 8391 8391 0 1443252 973356 139 3377 3796
	      GETACL: 0 0 0 0 0 0 0 0
	      SETACL: 0 0 0 0 0 0 0 0
	FS_LOCATIONS: 0 0 0 0 0 0 0 0
	RELEASE_LOCKOWNER: 0 0 0 0 0 0 0 0
	     SECINFO: 0 0 0 0 0 0 0 0
	 EXCHANGE_ID: 0 0 0 0 0 0 0 0
	CREATE_SESSION: 0 0 0 0 0 0 0 0
	DESTROY_SESSION: 0 0 0 0 0 0 0 0
	    SEQUENCE: 0 0 0 0 0 0 0 0
	GET_LEASE_TIME: 0 0 0 0 0 0 0 0
	RECLAIM_COMPLETE: 0 0 0 0 0 0 0 0
	   LAYOUTGET: 0 0 0 0 0 0 0 0
	GETDEVICEINFO: 0 0 0 0 0 0 0 0
	LAYOUTCOMMIT: 0 0 0 0 0 0 0 0
	LAYOUTRETURN: 0 0 0 0 0 0 0 0
	FREE_STATEID: 0 0 0 0 0 0 0 0
`

var np NFSPlugin

func TestGraphDefinition(t *testing.T) {
	graphdef := np.GraphDefinition()
	if len(graphdef) != 8 {
		t.Errorf("GetTempfilename: %d should be 8", len(graphdef))
	}
}

func TestParseMountStats(t *testing.T) {
	devices, stats, err := np.parseMountStats(strings.NewReader(str))
	if err != nil {
		t.Fatal(err)
	}

	if len(devices) == 0 {
		t.Fatal("no nfs mount points were found")
	}

	assert.EqualValues(t, devices[0], "mnt")
	assert.EqualValues(t, stats[fmt.Sprintf("ops.%s.read", devices[0])], float64(43026))
	assert.EqualValues(t, stats[fmt.Sprintf("ops.%s.write", devices[0])], float64(60192))
}

func TestFetchLastValues(t *testing.T) {
	os.Setenv("MACKEREL_PLUGIN_WORKDIR", "./test")
	stats, lastTime, err := np.fetchLastValues()
	if err != nil {
		t.Fatal(err)
	}

	assert.EqualValues(t, lastTime.Unix(), 1536731160)
	assert.EqualValues(t, stats["ops.mnt.read"], float64(42906))
	assert.EqualValues(t, stats["ops.mnt.write"], float64(60012))
}

func TestFormatValues(t *testing.T) {
	os.Setenv("MACKEREL_PLUGIN_WORKDIR", "./test")
	now := time.Unix(1536731220, 0)

	devices, stats, err := np.parseMountStats(strings.NewReader(str))
	if err != nil {
		t.Fatal(err)
	}

	lastStats, lastTime, err := np.fetchLastValues()
	if err != nil {
		t.Fatal(err)
	}

	metrics := np.formatValues(devices, stats, now, lastStats, lastTime)
	assert.EqualValues(t, metrics[fmt.Sprintf("ops.%s.read", devices[0])], float64(2))
	assert.EqualValues(t, metrics[fmt.Sprintf("ops.%s.write", devices[0])], float64(3))
}
