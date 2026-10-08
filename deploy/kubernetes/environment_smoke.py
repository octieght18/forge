#!/usr/bin/env python3
"""Real kind reconciliation tests. Mutates only fresh synthetic workload boundaries."""
import argparse
import copy
import importlib.util
import json
import os
from pathlib import Path
import secrets
import subprocess
import time

HERE=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location("native_smoke",HERE.parent/"native/smoke.py")
smoke=importlib.util.module_from_spec(spec)
spec.loader.exec_module(smoke)
GROUP="platform.forge.local"
FINALIZER=GROUP+"/environment-cleanup"
HOLD="tests.forge.local/hold"


class Proof:
    def __init__(self,root):
        self.root=root
        self.env=dict(os.environ,KUBECONFIG=str(root/"kubeconfig"))
        self.created=[]

    def kube(self,*args,data=None,okay=True):
        timeout=25
        for arg in args:
            if arg.startswith("--timeout=") and arg.endswith("s"):
                timeout=max(timeout,int(arg[len("--timeout="):-1])+5)
        r=subprocess.run(["kubectl","--context=kind-forge",*args],input=None if data is None else json.dumps(data).encode(),capture_output=True,env=self.env,timeout=timeout)
        if okay and r.returncode:
            raise RuntimeError("Kubernetes test operation failed; inspect private local diagnostics")
        return r

    def get(self,kind,name,namespace=None):
        args=["get",kind,name,"-o","json"]
        if namespace:args.extend(["-n",namespace])
        r=self.kube(*args,okay=False)
        if r.returncode:
            if b"NotFound" in r.stderr:return None
            raise RuntimeError("Kubernetes read failed")
        return json.loads(r.stdout)

    def wait(self,predicate,description,seconds=90):
        end=time.monotonic()+seconds
        while time.monotonic()<end:
            if predicate():return
            time.sleep(.5)
        raise RuntimeError("Timed out: "+description)

    def phase(self,name,phase):
        e=self.get("fenv",name)
        return bool(e and e.get("status",{}).get("phase")==phase)

    def apply(self,obj):return self.kube("apply","--validate=strict","-f","-",data=obj)
    def delete(self,name):
        if not name.startswith("workload-"):raise ValueError("Test deletion requires workload identity")
        self.kube("delete","fenv",name,"--ignore-not-found","--wait=false")

    def seed(self):
        values=json.loads((self.root/"secrets.json").read_text())
        workloads=[]
        for username in ("ahmad","second-owner"):
            token=smoke.login(self.root,username,values[username])
            w,_=smoke.api("POST","/api/v1/workloads",token,dict(name="f13-smoke-"+secrets.token_hex(6),description="Synthetic environment reconciliation validation"),expected=201)
            workloads.append(w)
        manifests=[]
        for w in workloads:
            name="workload-"+w["workload_id"]
            manifests.append(dict(apiVersion=GROUP+"/v1alpha1",kind="ForgeEnvironment",metadata=dict(name=name),spec=dict(workloadID=w["workload_id"],owner=w["owner"],profile="small-v1")))
            self.created.append(name)
        first,second=manifests
        name=first["metadata"]["name"];namespace="forge-w-"+first["spec"]["workloadID"]
        wrong=copy.deepcopy(first);wrong["spec"]["owner"]["subject"]=second["spec"]["owner"]["subject"]
        self.apply(wrong);self.wait(lambda:self.phase(name,"Failed"),"owner mismatch failed closed")
        assert self.get("namespace",namespace) is None
        self.delete(name);self.wait(lambda:self.get("fenv",name) is None,"failed intent cleanup")
        self.apply(first);self.wait(lambda:self.phase(name,"Ready"),"first boundary ready")
        cli=subprocess.run(["python3",str(HERE/"environment_cli.py"),"apply","--workload",first["spec"]["workloadID"],"--state-dir",str(self.root),"--token-file",str(self.root/"tokens/ahmad.json")],capture_output=True,timeout=25)
        assert cli.returncode==0, "operator intent wrapper failed"
        cli=subprocess.run(["python3",str(HERE/"environment_cli.py"),"status","--workload",first["spec"]["workloadID"],"--state-dir",str(self.root)],capture_output=True,timeout=25)
        assert cli.returncode==0 and json.loads(cli.stdout)["status"]["phase"]=="Ready"
        initial=self.get("fenv",name);initial_ns=self.get("namespace",namespace)
        for _ in range(3):self.apply(first)
        time.sleep(2)
        assert self.get("fenv",name)["metadata"]["uid"]==initial["metadata"]["uid"]
        assert self.get("namespace",namespace)["metadata"]["uid"]==initial_ns["metadata"]["uid"]
        self.schema(first)
        # Controller interruption leaves durable intent; replay repairs deleted children.
        self.kube("scale","-n","forge-local","deployment/environment-controller","--replicas=0")
        self.kube("wait","-n","forge-local","--for=delete","pod","-l","app=environment-controller","--timeout=20s")
        self.kube("delete","-n",namespace,"limitrange/containers","serviceaccount/worker")
        self.kube("patch","-n",namespace,"resourcequota/boundary","--type=merge","-p",json.dumps({"spec":{"hard":{"pods":"1"}}}))
        self.apply(second)
        second_name=second["metadata"]["name"]
        assert self.get("namespace","forge-w-"+second["spec"]["workloadID"]) is None
        self.kube("scale","-n","forge-local","deployment/environment-controller","--replicas=1")
        self.kube("rollout","status","-n","forge-local","deployment/environment-controller","--timeout=120s")
        self.wait(lambda:self.get("limitrange","containers",namespace) is not None and self.get("serviceaccount","worker",namespace) is not None and self.get("resourcequota","boundary",namespace)["spec"]["hard"]["pods"]=="2","partial creation/drift repair")
        self.wait(lambda:self.phase(second_name,"Ready"),"second owner boundary")
        for m in manifests:
            ns="forge-w-"+m["spec"]["workloadID"]
            for account in ("default","worker"):
                assert self.get("serviceaccount",account,ns)["automountServiceAccountToken"] is False
            who="system:serviceaccount:"+ns+":worker"
            assert self.kube("auth","can-i","get","secrets","-n","forge-local","--as="+who,okay=False).stdout.strip()==b"no"
            assert self.kube("auth","can-i","create","pods","-n",ns,"--as="+who,okay=False).stdout.strip()==b"no"
        self.admission(namespace)
        # Persist proof privately so whole-node stop/start can verify exact identities.
        proof=dict(manifests=manifests,environment_uids={m["metadata"]["name"]:self.get("fenv",m["metadata"]["name"])["metadata"]["uid"] for m in manifests},namespace_uids={"forge-w-"+m["spec"]["workloadID"]:self.get("namespace","forge-w-"+m["spec"]["workloadID"])["metadata"]["uid"] for m in manifests})
        path=self.root/"f13-proof.json"
        with path.open("w") as out:
            os.fchmod(out.fileno(),0o600);json.dump(proof,out)
        print(json.dumps(dict(phase="seed",owner_mismatch=True,duplicate_identity=True,immutable_schema=True,controller_restart=True,partial_creation=True,drift_repair=True,admission=True,two_owner_boundaries=True,worker_rbac_denied=True)))

    def schema(self,manifest):
        current=self.get("fenv",manifest["metadata"]["name"])
        for change in ({"profile":"large"},{"owner":{"subject":"someone-else"}},{"image":"arbitrary"}):
            candidate=copy.deepcopy(current)
            for key,value in change.items():
                if key=="owner":candidate["spec"]["owner"].update(value)
                else:candidate["spec"][key]=value
            # replace supports strict validation; patch has no --validate flag.
            result=self.kube("replace","--dry-run=server","--validate=strict","-f","-",data=candidate,okay=False)
            assert result.returncode!=0 and any(reason in result.stderr.lower() for reason in (b"invalid",b"immutable",b"unknown field")), "API schema rejection was not observed"
        bad=copy.deepcopy(manifest);bad["metadata"]["name"]="different-name"
        result=self.kube("create","--dry-run=server","-f","-",data=bad,okay=False)
        assert result.returncode!=0 and b"invalid" in result.stderr.lower(), "API name validation was not observed"

    def admission(self,namespace):
        image=json.loads((HERE/"images.json").read_text())["postgres"]
        pod=dict(apiVersion="v1",kind="Pod",metadata=dict(name="f13-probe",namespace=namespace),spec=dict(serviceAccountName="worker",restartPolicy="Never",containers=[dict(name="probe",image=image,command=["sh","-c","sleep 120"],securityContext=dict(runAsNonRoot=True,runAsUser=999,allowPrivilegeEscalation=False,capabilities=dict(drop=["ALL"]),seccompProfile=dict(type="RuntimeDefault"))) ]))
        unsafe=copy.deepcopy(pod);unsafe["spec"]["containers"][0]["securityContext"]["privileged"]=True
        assert self.kube("create","--dry-run=server","-f","-",data=unsafe,okay=False).returncode!=0
        huge=copy.deepcopy(pod);huge["spec"]["containers"][0]["resources"]=dict(requests=dict(cpu="2",memory="64Mi"),limits=dict(cpu="2",memory="512Mi"))
        assert self.kube("create","--dry-run=server","-f","-",data=huge,okay=False).returncode!=0
        self.kube("create","-f","-",data=pod)
        admitted=self.get("pod","f13-probe",namespace)
        c=admitted["spec"]["containers"][0]
        assert c["resources"]==dict(requests=dict(cpu="100m",memory="64Mi"),limits=dict(cpu="500m",memory="512Mi"))
        assert not any(v.get("projected",{}).get("sources",[{}])[0].get("serviceAccountToken") for v in admitted["spec"].get("volumes",[]))
        two=copy.deepcopy(pod);two["metadata"]["name"]="f13-probe-two";self.kube("create","-f","-",data=two)
        self.wait(lambda:self.get("resourcequota","boundary",namespace).get("status",{}).get("used",{}).get("pods")=="2","quota usage recorded")
        three=copy.deepcopy(pod);three["metadata"]["name"]="f13-probe-three"
        assert self.kube("create","-f","-",data=three,okay=False).returncode!=0
        self.kube("delete","pod/f13-probe","pod/f13-probe-two","-n",namespace,"--wait=true","--timeout=20s","--grace-period=1")

    def verify(self):
        proof=json.loads((self.root/"f13-proof.json").read_text())
        for m in proof["manifests"]:
            name=m["metadata"]["name"];ns="forge-w-"+m["spec"]["workloadID"]
            self.created.append(name);self.wait(lambda:self.phase(name,"Ready"),"retained boundary ready")
            assert self.get("fenv",name)["metadata"]["uid"]==proof["environment_uids"][name]
            assert self.get("namespace",ns)["metadata"]["uid"]==proof["namespace_uids"][ns]
        name=proof["manifests"][0]["metadata"]["name"];ns="forge-w-"+proof["manifests"][0]["spec"]["workloadID"]
        # The test owns this hold; controller must neither remove nor bypass it.
        cm=dict(apiVersion="v1",kind="ConfigMap",metadata=dict(name="cleanup-hold",namespace=ns,finalizers=[HOLD]))
        self.apply(cm)
        # A synthetic static CSI reference has no real provisioned disk or Pod.
        # Even an orphaned PV reference must prevent namespace deletion.
        pvname="forge-f13-probe-"+proof["manifests"][0]["spec"]["workloadID"]
        volume=dict(apiVersion="v1",kind="PersistentVolume",metadata=dict(name=pvname),spec=dict(capacity=dict(storage="1Mi"),accessModes=["ReadWriteOnce"],persistentVolumeReclaimPolicy="Retain",storageClassName="",claimRef=dict(namespace=ns,name="synthetic-absent-claim"),csi=dict(driver="tests.forge.local",volumeHandle=pvname)))
        self.kube("create","-f","-",data=volume)
        cli=subprocess.run(["python3",str(HERE/"environment_cli.py"),"delete","--workload",proof["manifests"][0]["spec"]["workloadID"],"--state-dir",str(self.root)],capture_output=True,timeout=25)
        assert cli.returncode==0, "operator deletion wrapper failed"
        self.wait(lambda:self.phase(name,"Deleting"),"deleting status")
        self.wait(lambda:self.get("fenv",name).get("status",{}).get("conditions",[{}])[0].get("reason")=="PersistentStoragePresent","storage prevents namespace deletion")
        assert not self.get("namespace",ns)["metadata"].get("deletionTimestamp")
        self.kube("delete","persistentvolume",pvname,"--wait=true","--timeout=20s")
        self.wait(lambda:bool(self.get("configmap","cleanup-hold",ns)["metadata"].get("deletionTimestamp")),"namespace controller cleanup")
        time.sleep(2)
        assert HOLD in self.get("configmap","cleanup-hold",ns)["metadata"]["finalizers"]
        assert FINALIZER in self.get("fenv",name)["metadata"]["finalizers"]
        self.kube("patch","configmap/cleanup-hold","-n",ns,"--type=json","-p",json.dumps([dict(op="test",path="/metadata/finalizers/0",value=HOLD),dict(op="remove",path="/metadata/finalizers/0")]))
        self.wait(lambda:self.get("fenv",name) is None and self.get("namespace",ns) is None,"finalized exact namespace")
        for m in proof["manifests"][1:]:
            name=m["metadata"]["name"];ns="forge-w-"+m["spec"]["workloadID"]
            self.delete(name);self.wait(lambda:self.get("fenv",name) is None and self.get("namespace",ns) is None,"normal cleanup")
        assert self.get("namespace","forge-local") is not None
        assert self.get("persistentvolume","forge-local-postgres")["spec"]["persistentVolumeReclaimPolicy"]=="Retain"
        print(json.dumps(dict(phase="verify",node_restart_identity=True,persistent_storage_blocked=True,foreign_finalizer_preserved=True,cleanup_completed=True,shared_database_protected=True)))


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument("phase",choices=["seed","verify"]);p.add_argument("--state-dir",type=Path,default=Path.home()/".local/share/forge-kubernetes");args=p.parse_args()
    if os.geteuid()==0:p.error("Run as the unprivileged state owner")
    proof=Proof(args.state_dir)
    if args.phase=="seed":proof.seed()
    else:proof.verify()


if __name__=="__main__":main()
