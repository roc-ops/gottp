"""Records testdata/python.json for internal/call's tests from Python TTP
0.10.1 (ttp.utils.get_attributes) and difflib.get_close_matches.

Usage: python -I gen_python.py OUT.json  (a Python with ttp==0.10.1)
"""
import difflib, json, random, sys
from ttp.utils.get_attributes import get_attributes
cands = ["to_int","to_str","to_list","to_float","to_ip","to_net","contains","containsall","exclude","excludeall","equal","notequal","startswith_re","endswith_re","contains_re","exclude_re","notstartswith_re","notendswith_re","joinmatches","sformat","replaceall","resub","resuball","is_ip","is_private","cidr_match","truncate","unrange","item","macro","lookup","rlookup","gpvlookup","record","set","let","count","void","geoip_lookup","rdns","dns","mac_eui","uptimeparse","join","split","upper","lower","strip","lstrip","rstrip","replace","title","append","prepend","print","raise","del","itemize","items2dict","extract_commands","test","is_equal","traverse","dict_to_list"]
random.seed(7)
words = ["to_itn","contians","extract_comands","is_eqal","nosuchfn","joinmatch","uper","lowr","contain","sformt","totally","a","x","to_in","itm","repalce","rstrp","lookp","recrod","dict_to_lst","t","contains_r","excludeal","to_ints","sett","del_","printt"]
for _ in range(40):
    w=random.choice(cands); w=list(w)
    for _ in range(random.randint(1,3)):
        op=random.randint(0,2); p=random.randrange(len(w))
        if op==0: del w[p]
        elif op==1: w.insert(p,random.choice("abcdefghijklmnopqrstuvwxyz_"))
        else: w[p]=random.choice("abcdefghijklmnopqrstuvwxyz_")
        if not w: w=['q']
    words.append("".join(w))
out={"candidates":cands,"guess":{w:difflib.get_close_matches(w,cands,n=3,cutoff=0.8) for w in words}}
calls=['upper','upper()','re("\\\\d+")','re(r"\\d+")','sformat("v={}")','contains(vlan, ip)','x(1, -2, 3.5, True, false, NONE, None)',
 'lookup(aux, add_field="info")','f([1, "a", [2,3]], {"k": [1]})','f("a" "b")','joinmatches(",")','f(1e20, 1e-5, 100.0, 0.1)',
 'item ("x")','f(a=1, b="2")','f(())','f((1))','f((1,))','set("val")','f(r"\\\\")','f("\\x41\\n")', 'f(1,)', 'contains("a,b")']
res={}
for c in calls:
    a=get_attributes(c)[0]
    def conv(v):
        if isinstance(v,tuple): return [conv(i) for i in v]
        if isinstance(v,list): return [conv(i) for i in v]
        if isinstance(v,dict): return {str(k):conv(x) for k,x in v.items()}
        return v
    res[c]={"name":a["name"],"args":conv(list(a["args"])),"kwargs":conv(a["kwargs"]),"strs":[str(x) if not isinstance(x,(tuple,)) else str(list(x)) for x in a["args"]]}
out["calls"]=res
json.dump(out,open(sys.argv[1],"w"),indent=1)
