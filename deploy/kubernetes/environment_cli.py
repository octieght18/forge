#!/usr/bin/env python3
"""Submit operator environment intent; the controller performs reconciliation."""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import urllib.error
import urllib.request
from environments import GROUP, UUID


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self,request,fp,code,msg,headers,url):
        return None


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument("action",choices=["apply","status","delete"])
    p.add_argument("--workload",required=True)
    p.add_argument("--state-dir",type=Path,default=Path.home()/".local/share/forge-kubernetes")
    p.add_argument("--token-file",type=Path)
    a=p.parse_args()
    if not re.fullmatch(UUID,a.workload):p.error("Workload must be a lowercase UUID v4")
    root=a.state_dir
    if root.is_symlink() or root.stat().st_uid!=os.getuid() or root.stat().st_mode & 0o077:
        p.error("Use your private Kubernetes installation directory")
    marker=json.loads((root/"installation.json").read_text())
    if marker!=dict(format=1,uid=os.getuid(),root=str(root),cluster="forge"):
        p.error("Kubernetes installation identity mismatch")
    name="workload-"+a.workload
    args=["kubectl","--context=kind-forge"]
    data=None
    if a.action=="apply":
        tokenfile=a.token_file or root/"tokens/access.json"
        if tokenfile.is_symlink() or tokenfile.stat().st_uid!=os.getuid() or tokenfile.stat().st_mode & 0o077 or tokenfile.stat().st_size>128*1024:
            p.error("Token file must be private, owned by this user and bounded")
        token=json.loads(tokenfile.read_text())["access_token"]
        req=urllib.request.Request("http://127.0.0.1:8081/api/v1/workloads/"+a.workload,headers={"Authorization":"Bearer "+token})
        try:
            with urllib.request.build_opener(NoRedirect()).open(req,timeout=5) as response:
                payload=response.read(65537)
        except urllib.error.HTTPError as error:
            p.error("Authenticated workload read failed (HTTP "+str(error.code)+"); log in or check ownership")
        if len(payload)>65536:p.error("Workload response exceeds the supported bound")
        w=json.loads(payload)
        if w["workload_id"]!=a.workload:p.error("Returned workload identity mismatch")
        data=json.dumps(dict(apiVersion=GROUP+"/v1alpha1",kind="ForgeEnvironment",metadata=dict(name=name),spec=dict(workloadID=a.workload,owner=w["owner"],profile="small-v1"))).encode()
        args.extend(["apply","--validate=strict","-f","-"])
    elif a.action=="status":args.extend(["get","fenv",name,"-o","json"])
    else:args.extend(["delete","fenv",name,"--wait=false","--ignore-not-found"])
    r=subprocess.run(args,input=data,env=dict(os.environ,KUBECONFIG=str(root/"kubeconfig")),timeout=20)
    if r.returncode:raise SystemExit(r.returncode)


if __name__=="__main__":main()
