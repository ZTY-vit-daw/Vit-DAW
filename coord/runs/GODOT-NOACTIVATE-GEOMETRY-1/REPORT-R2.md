# 405 R2 runtime — first executed five-step candidate failed; no rerun

Owner /root/restore403; supervisor /root final review pending. Authorization sharedmaineab989fe/card/resource read before RUNBOOK-R2 freeze. Tested frontend87a5359 and scriptc43d66ad unchanged (mainHEAD2dbb76a6). No code/standard change, postshow correction or replacement run. Allpriorfailures/evidence preserved.

## Outcome

Configure/build0; synthetic host21488/probe40 candidate_fail; runner/wrapper1. ExactlyoneGodot child13400/thread29584, exactEngineHWND919086/gen1/nonce901adc49d5cd4b55a464a4f76a12edd9. Run11:31:26.9807890Z–11:32:07.9009355Z (<41s/15min). This time the ownPMv2guard2491438 wasactuallyforeground, SetForegroundWindow returned1 andprechildbaselineconfirmed. Externalcondition stopped immediatelyaftercandidatefailure, notexercisedbythisbudget. No retry.

## Strict domain and diagnostics

Exactengine/nativeobserverassociation, load0/safe_armedidentity/zeroerrors passed beforemutation. Monitor65608; P/T independentGetMonitorInfo work/monitor both[0,0,2560,1440] exact; targetSYSTEM_AWARE/Tcontext equalitytrue, guardPMv2; GetDpiForSystem(T)=childDPI=guardDPI168. New same_monitor_system_dpi_exact_read contractaccepted; notfallbackconversion.

All8diagnosticcornercalls were independentlyexecuted: bothcorners ofwork/monitor forwardandinverseeachBOOL0, rawLastError0, output_valid0. Inputs/unchangedoutputs andcontextrecordedseparately; summary4groups0. Per-monitorAPIs do notdocumentGetLastError, so raw0meansnoattributablecause, notsuccess. This clarifiesactualpointAPIoutcomesforR2 withoutretroactivelyclaimingR1exactcause orusingfailuresasapassinput. Geometryadmission cameonlyfromstrictindependentdomainfacts.

## Five steps reached and first-cause evidence

ParentexactQPCintervals/arguments/returns showallfivetargetsteps once, inT:

1. addEX_NOACTIVATE succeeded;
2. hideNOACTIVATE/NOZORDER succeeded, childhidden/iconic;
3. SetWindowPlacement(SW_HIDE,flags0) returned1; copiednormalrectbecame[0,0,2560,1440] whilehidden/iconic;
4. hiddenSetWindowPosafterownedguard2491438 returned1; nativeCHANGEDseq34/35 confirmprevious=2491438; hiddenpremise/readbackpassed;
5. soleShowWindow(SW_SHOWNOACTIVATE) returnpreviousvisibility0, actualvisible1/noniconic/nonzoomed/nonTOPMOST, outerrect[0,0,2560,1440], client2536×1376. NoWS_MINIMIZEclearing, activatingrestore/maximizeorpostshowresize/zrepair.

ShowQPCbegin1156813617299/end1156813803877. Withinthatinterval the target-nativeCHANGINGseq38 entry/41after alreadyproposedinsert_after788060(flags33136), differentfromguard2491438; actualhiddenpreviousatthatentrywasstillguard. Firstordinarysnapshotseq44/QPC1156813746435 hasworkarearect[0,0,2560,1440], noniconic/nonzoomed/nonTOPMOST, foregroundstillguard2491438/focus0, but previous=788060. CHANGEDseq50/51 confirmsinsert_after/previous788060. Alllaterordinaryrecordsremaintargetrectandprevious788060, notguard.

The frozen gate requiredguardastheexactimmediatezpredecessor beforeandafterrestore, andthiswasviolatedduringtheShowWindowtransaction. Parentphysicalauditseq82 passedrect, thengeometryNativeNormal'simmediatepreviouscheckrejected, leadinggenericcandidate_fail40. Parentnormal/visible/iconic/FGfactswereotherwisegood; no2sgodot_intervalorstableloopcompleted. The runtimecode didnotreachitsfinalgeometryTrace verdict, so independenttargettraceanchors—notarun-timegeometryTracePASS/FAILlabel—establishthewrongpredecessorfact.

**Do not overclaimthezfinding:** snapshotsrecordonlyprevious/nextandproposals, notacompletetop-levelzranklistatthatmoment. previous≠guardprovesthefrozenimmediateadjacencygatefailed; byitselfitdoesnotprovewhetherguardwasstillhigherelsewhereinthechainorwhetherchildappearedaboveguard. HWND788060ownership/class/identitywasnotcollected, so donotguesswhatapplicationitwasorwhichWindows/Godotcomponentcausedtheproposal. Nootherwindowwasmanipulatedtoinvestigate.

Separately, passiveengineobserverimmediatelyaftershowlogssize_changedseq23andlatersamplesbothWindow/Displaymode4, width2536/height1376, focused0; nottherequirednormalmode0. NativeIsZoomed0/style/rectfactsarestillseparatefromengineenum4. OfficialGodot4.6.1sourcealreadyreviewedhasWM_WINDOWPOSCHANGED logic settingfullscreenwhenordinarywindowrectequalsscreenrect (refsdisplay_server_windows.cpp~5867); thisrun'swork==monitor[0,0,2560,1440]isconsistentwiththatbranch. Itisastaticexplanatoryanchor, notinstrumentedbranchproof or permissiontoshiftgeometry/shrinktarget/excludemode. No2sengine0claimcanbemade. Bothzadjacencyandmode0criteriawouldneedresolutionbeforeproductuse; currentfrozenrunisfailed.

## Target trace and graceful closure

Originalnativeoutput84contiguousseq: initial1 +41entry/afterpairs82 +removerevidence1. IndependentLIFOcheckerrors[], maxdepth2, allfailures/dropped/IO0. Allordinarynativeouterrectsequalworktarget; firstordinaryoccursinsideshowinterval. ChildrawFOREGROUND0/focus0, guardbaselinekept. independent_crosscheck.json preservesfiveQPCmutations,8diagnostics,firstordinary,proposalrecords/modefactsandnaturalexit. Thisisameasuredcandidatecounterexampleunderfrozenz/enginegates, notanobserverfailureorfullproductverdict.

ParentCREATE_NEWexactnoncestop -> ownfixtureunload0 -> removed_verified/shutdowntrue, depth0/pending0/zeroerror/drop/IO, drainhelper/classremoved -> acksameidentity/notloaded ->actualnaturalchild0. Parentgraceful_teardowntrue/forced_teardown0/wait0. SuccessfulcleanupdoesnotconvertcandidatefailureintoPASS. NoTerminateProcessneeded; noforeignprocess/windowwrites.

## Provenance/hygiene and stop

RawRUNBOOK/commandexits/sourceHEAD/status/diff/GodotGUIversion/hash/ABI/DLL/probehash/nonceidentity/loader/parent+target+engineJSONL/stopackareunder runtime_02. Current405lease+ownerand<=60sidlechecksrecordedbeforebuild/afterlongbuild/beforechild. Final11:32:07.8988571Z allVit/Godot/probeprocesses[], TCP5555/5556/7878/7879/8787/9333[], UDP4445[]. Fullrecentsbefore.json andbefore/afterSHA807226F4AF3FC85C2B99BFD2FB827C1583E7C991826BDE5F0B4344A6DC13E353 agree. SourceunchangedandnofullDAW/userproject/kernel/agent/LLM/CUA/input/screenshot. OriginalbyteZIP/SHAretained,expandedfileskept; binaries/build/cacheexcludedfromcommit/archive.

R2budgetstoppedwithfirstanomaly, externalnotrun, noopportunisticAPIvariant orcorrection. Supportedfacts: strictsameDPIreadcontract, hiddennormalplacementstage, actualfirstordinaryworkareageometry, targetzproposalchange, engine4not0, noFGandnaturalcleanup. Unproven/failed: frozenpostrestoreguardadjacency andengine0≥2s; completezchain/precisetransitionauthorunknown. Nosuccessful405candidateorproduct/userhandtestrelease. Supervisorownsnextdiagnosis/designscope/resourceclosure;executionstopsafterevidencesubmission.
