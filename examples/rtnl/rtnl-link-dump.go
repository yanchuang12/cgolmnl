package main

/*
#include <stdlib.h>
#include <sys/socket.h>
#include <linux/if.h>
#include <linux/rtnetlink.h>
*/
import "C"

import (
	"fmt"
	mnl "github.com/chamaken/cgolmnl"
	"os"
	"syscall"
	"time"
)

func data_attr_cb(attr *mnl.Nlattr, data interface{}) (int, syscall.Errno) {
	tb := data.(map[uint16]*mnl.Nlattr)
	attr_type := attr.GetType()

	if err := attr.TypeValid(C.IFLA_MAX); err != nil {
		return mnl.MNL_CB_OK, 0
	}

	switch attr_type {
	case C.IFLA_ADDRESS:
		if err := attr.Validate(mnl.MNL_TYPE_BINARY); err != nil {
			fmt.Fprintf(os.Stderr, "mnl_attr_validate: %s\n", err)
			return mnl.MNL_CB_ERROR, err.(syscall.Errno)
		}
	case C.IFLA_MTU:
		if err := attr.Validate(mnl.MNL_TYPE_U32); err != nil {
			fmt.Fprintf(os.Stderr, "mnl_attr_validate: %s\n", err)
			return mnl.MNL_CB_ERROR, err.(syscall.Errno)
		}
	case C.IFLA_IFNAME:
		if err := attr.Validate(mnl.MNL_TYPE_STRING); err != nil {
			fmt.Fprintf(os.Stderr, "mnl_attr_valudate: %s\n", err)
			return mnl.MNL_CB_ERROR, err.(syscall.Errno)
		}
	}
	tb[attr_type] = attr
	return mnl.MNL_CB_OK, 0
}

func data_cb(nlh *mnl.Nlmsg, data interface{}) (int, syscall.Errno) {
	tb := make(map[uint16]*mnl.Nlattr, C.IFLA_MAX+1)

	ifm := (*Ifinfomsg)(nlh.Payload())
	fmt.Printf("index=%d type=%d flags=%d family=%d ", ifm.Index, ifm.Type, ifm.Flags, ifm.Family)

	if ifm.Flags&C.IFF_RUNNING == C.IFF_RUNNING {
		fmt.Printf("[RUNNING] ")
	} else {
		fmt.Printf("[NOT RUNNING] ")
	}

	nlh.Parse(SizeofIfinfomsg, data_attr_cb, tb)
	if tb[C.IFLA_MTU] != nil {
		fmt.Printf("mtu=%d ", tb[C.IFLA_MTU].U32())
	}
	if tb[C.IFLA_IFNAME] != nil {
		fmt.Printf("name=%s ", tb[C.IFLA_IFNAME].Str())
	}

	if tb[C.IFLA_ADDRESS] != nil {
		hwaddr := *(*[8]byte)(tb[C.IFLA_ADDRESS].Payload())
		// hwaddr := tb[C.IFLA_ADDRESS].PayloadBytes()
		fmt.Printf("hwaddr=")
		var i uint16
		for i = 0; i < tb[C.IFLA_ADDRESS].PayloadLen(); i++ {
			fmt.Printf("%.2x", hwaddr[i]&0xff)
			if i+1 != tb[C.IFLA_ADDRESS].PayloadLen() {
				fmt.Printf(":")
			}
		}
	}
	fmt.Printf("\n")
	return mnl.MNL_CB_OK, 0
}

func main() {
	buf := make([]byte, mnl.MNL_SOCKET_BUFFER_SIZE)
	nlh, err := mnl.NewNlmsgBytes(buf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nlmsg_put_header: %s\n", err)
		os.Exit(C.EXIT_FAILURE)
	}

	nlh.Type = C.RTM_GETLINK
	nlh.Flags = C.NLM_F_REQUEST | C.NLM_F_DUMP
	seq := uint32(time.Now().Unix())
	nlh.Seq = seq
	rt := (*Rtgenmsg)(nlh.PutExtraHeader(SizeofRtgenmsg))
	rt.Family = C.AF_PACKET

	nl, err := mnl.NewSocket(C.NETLINK_ROUTE)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnl_socket_open: %s\n", err)
		os.Exit(C.EXIT_FAILURE)
	}
	defer nl.Close()

	if err = nl.Bind(0, mnl.MNL_SOCKET_AUTOPID); err != nil {
		fmt.Fprintf(os.Stderr, "mnl_socket_bind: %s\n", err)
		os.Exit(C.EXIT_FAILURE)
	}
	portid := nl.Portid()

	if _, err = nl.SendNlmsg(nlh); err != nil {
		fmt.Fprintf(os.Stderr, "mnl_socket_sendto: %s\n", err)
		os.Exit(C.EXIT_FAILURE)
	}

	ret := mnl.MNL_CB_OK
	for ret > mnl.MNL_CB_STOP {
		nrcv, err := nl.Recvfrom(buf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mnl_socket_recvfrom: %s\n", err)
			os.Exit(C.EXIT_FAILURE)
		}
		ret, err = mnl.CbRun(buf[:nrcv], seq, portid, data_cb, nil)
	}
	if ret < mnl.MNL_CB_STOP {
		fmt.Fprintf(os.Stderr, "mnl_cb_run: %s\n", err)
		os.Exit(C.EXIT_FAILURE)
	}
}
