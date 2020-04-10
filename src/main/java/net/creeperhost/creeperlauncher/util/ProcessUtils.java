package net.creeperhost.creeperlauncher.util;

import java.io.File;
import java.io.IOException;
import java.nio.file.Files;

public class ProcessUtils {
    public static String GetUnixExec(long procId) throws IOException {
        File fn = new File("/proc/"+procId+"/cmdline");
        if(fn.exists())
        {
            String cmdline = new String(Files.readAllBytes(fn.toPath()));
            int lio = cmdline.lastIndexOf("/");
            lio = (lio > 0) ? lio+1 : lio;
            cmdline = cmdline.substring(lio);
            lio = cmdline.indexOf('\000');
            cmdline = cmdline.substring(0, lio);
            return cmdline;
        }
        return null;
    }
}
