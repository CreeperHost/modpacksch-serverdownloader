package net.creeperhost.ftbserverdownloader;

import com.google.gson.Gson;
import com.google.gson.JsonArray;
import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import net.creeperhost.creeperlauncher.CreeperLogger;
import net.creeperhost.creeperlauncher.api.DownloadableFile;
import net.creeperhost.creeperlauncher.install.tasks.DownloadTask;

import javax.net.ssl.SSLContext;
import javax.net.ssl.SSLParameters;
import java.io.*;
import java.net.*;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.time.Duration;
import java.util.*;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.Executor;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicLong;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

public class Main {
    public static AtomicLong overallBytes = new AtomicLong(0);
    public static AtomicLong currentBytes = new AtomicLong(0);
    public static AtomicInteger dlnum = new AtomicInteger();
    public static Path installPath;
    public static String verString = "@VERSION@";
    public static boolean generateStart = true;
    private static ArrayList<ServerPack> packs = new ArrayList<ServerPack>();
    public static void main(String[] args) {
        long expectedPack = 0;
        boolean autoInstall = false;
        long expectedVer = 0;
        String installPack = null;
        String installVer = null;
        if(args.length > 0)
        {
            installPack = args[0];
        }
        if(args.length > 1)
        {
            installVer = args[1];
        }
        String execName = "unknownInstaller";
        try {
            File execFile = new File(Main.class.getProtectionDomain().getCodeSource().getLocation().getPath());
            if(execFile == null)
            {
                execName = ProcessHandle.current().info().command().get();
            } else {
                execName = execFile.getName();
                if (execName == null) {
                    execName = ProcessHandle.current().info().command().get();
                }
            }
            final String regex = "\\w+\\_(\\d+)\\_(\\d+).*";
            final Pattern pattern = Pattern.compile(regex);
            final Matcher matcher = pattern.matcher(execName);
            if (matcher.find()) {
                installPack = matcher.group(1);
                installVer = matcher.group(2);
            } else {
                String execLine = ProcessHandle.current().info().commandLine().get();
                if(execLine != null) {
                    final Matcher matcher2 = pattern.matcher(execLine);
                    if (matcher2.find()) {
                        installPack = matcher.group(1);
                        installVer = matcher.group(2);
                    }
                }
            }
        } catch (Exception ignored) {}
        boolean search = false;
        installPath = Paths.get("./");

        String argName = null;
        HashMap<String, String> Args = new HashMap<String, String>();
        for(String arg : args)
        {
            if(arg.substring(0,2).equals("--")) {
                argName = arg.substring(2);
                Args.put(argName, "");
            }
            if(argName != null)
            {
                if(!argName.equals(arg.substring(2))) {
                    if (Args.containsKey(argName)) {
                        Args.remove(argName);
                    }
                    Args.put(argName, arg);
                    argName = null;
                }
            }
        }
        if(installPack != null && installVer != null)
        {
            autoInstall = (!Args.containsKey("auto"));
        }
        generateStart = (!Args.containsKey("noscript"));
        System.out.println("                      _                  _              _     ");
        System.out.println("                     | |                | |            | |    ");
        System.out.println("  _ __ ___   ___   __| |_ __   __ _  ___| | _____   ___| |__  ");
        System.out.println(" | '_ ` _ \\ / _ \\ / _` | '_ \\ / _` |/ __| |/ / __| / __| '_ \\ ");
        System.out.println(" | | | | | | (_) | (_| | |_) | (_| | (__|   <\\__ \\| (__| | | |");
        System.out.println(" |_| |_| |_|\\___/ \\__,_| .__/ \\__,_|\\___|_|\\_\\___(_)___|_| |_|");
        System.out.println("                       | |                                    ");
        System.out.println("                       |_|                                    ");
        System.out.println("              modpacks.ch server downloader - build "+verString);
        System.out.println("");
        if(Args.containsKey("help"))
        {
            System.out.println("Usage:");
            System.out.println(execName+" - Start an interactive download and install.");
            System.out.println(execName+" searchterm - Search for a modpack and start interactive download and install.");
            System.out.println(execName+" <packid> - Download and install the latest Release version of a modpack by id.");
            System.out.println(execName+" <packid> <versionid> - Download and install a specific version of a modpack by id and version id.");
            System.out.println("");
            System.out.println("Additional arguments:");
            System.out.println("--help - Print this help information.");
            System.out.println("--path - Specify an install path instead of current working directory.");
            System.out.println("--auto - Automatically install a pack for filename based pack installs.");
            System.out.println("--noscript - Don't generate start.sh and start.bat files.");
            System.out.println("--latest - Ignore any specific modpack version id provided and always download the latest.");
            System.out.println("Example: "+execName+" 47 295 --path /home/ftb/omnia");
            System.out.println("");
            System.exit(0);
        }
        if(Args.containsKey("path"))
        {
            Path tmpPath = Path.of(Args.get("path"));
            if(!tmpPath.toFile().exists())
            {
                System.out.println("Requested install path '"+tmpPath.toAbsolutePath().toString()+"' does not exist.");
                System.exit(-5);
            } else {
                installPath = tmpPath;
            }
        }
        boolean latest = true;
	    try
        {
            //Do we have an pack ID or are we searching for a pack?
            expectedPack = Long.parseLong(installPack);
        } catch(Exception ignored)
        {
            search = true;
        }
	    if(!search) {
            if (args.length > 1||installVer != null) {
                try {
                    //Do we have a version ID or are we just grabbing the latest?
                    expectedVer = Long.parseLong(installVer);
                    latest = false;
                } catch (Exception ignored) {
                    latest = true;
                }
            }
        }
	    if(Args.containsKey("latest"))
        {
            latest = true;
        }
	    if(search) {
            String term = null;
            if(args.length > 0) {
                try {
                    term = URLEncoder.encode(args[0], "UTF-8");
                } catch (UnsupportedEncodingException e) {
                    e.printStackTrace();
                }
            }
            int ch = 0;
            if(term == null)
            {
                System.out.println("Please enter a search term to view modpacks (Minimum 4 characters)");
                try {
                    BufferedReader reader = new BufferedReader( new InputStreamReader( System.in ) );
                    String input = new String();
                    while( input.length() < 1 ){

                        input = reader.readLine();
                    }
                    if(input.length() < 4)
                    {
                        System.out.println("Please try again, term too short.");
                        System.exit(-1);
                    } else {
                        try {
                            term = URLEncoder.encode(input, "UTF-8");
                        } catch (UnsupportedEncodingException e) {
                            System.out.println("Please try again, term contains invalid characters.");
                            System.exit(-2);
                        }
                    }
                }
                catch(Exception ignored){}
            }
            System.out.println("Searching for '"+term+"'...");
            ArrayList<CompletableFuture> futures = new ArrayList<>();
            HttpClient wclient = HttpClient.newHttpClient();
            HttpRequest request = HttpRequest.newBuilder()
                    .uri(URI.create("https://api.modpacks.ch/public/modpack/search/8?term=" + term))
                    .build();
            wclient.sendAsync(request, HttpResponse.BodyHandlers.ofString())
                    .thenApply(HttpResponse::body)
                    .thenAccept((String data) -> {
                        Gson gson = new Gson();
                        JsonObject apiresp = gson.fromJson(data, JsonObject.class);
                        JsonArray packs = apiresp.getAsJsonArray("packs");
                        for (JsonElement pack : packs) {
                            long packId = pack.getAsLong();
                            ServerPack tmp = new ServerPack(packId);
                            futures.add(CompletableFuture.runAsync(() -> {
                                tmp.downloadManifest();
                                Main.packs.add(tmp);
                            }));
                        }
                    }).join();
            CompletableFuture<Void> combinedFuture = CompletableFuture.allOf(
                    futures.toArray(new CompletableFuture[0])).exceptionally((t) ->
                    {
                        t.printStackTrace();
                        return null;
                    }
            );
            combinedFuture.join();


            int num = 1;
            System.out.println("Please choose a pack from the options below:");
            for (ServerPack pack : packs) {
                System.out.println(num+") " + pack.name);
                num++;
            }
            ServerPack selectedPack = null;
            ch = 0;
            while (true)
            {
                try {
                    if (!((ch = System.in.read()) != -1)) break;
                } catch (IOException e) {
                    e.printStackTrace();
                }
                if (ch != '\n' && ch != '\r')
                {
                    try {
                        int tpack = (Integer.parseInt(String.valueOf((char) ch)) - 1);
                        if (tpack >= 0 && tpack <= (packs.size() - 1)) {
                            selectedPack = packs.get(tpack);
                            break;
                        } else {
                            System.out.println("Invalid selection, please try again.");
                        }
                    } catch (Exception ignored)
                    {
                        System.out.println("Invalid selection, please try again.");
                    }
                }
            }
            System.out.println("Selected '"+selectedPack.name+"'...");
            num = 1;
            System.out.println("Please select a version below:");
            Collections.reverse(selectedPack.versions);
            for(ServerVersion version : selectedPack.versions)
            {
                System.out.println(num+") " + version.name + " ["+ version.type + "]");
                num++;
                if(num > 9) break;
            }
            ServerVersion selectedVersion = null;
            ch = 0;
            while (true)
            {
                try {
                    if (!((ch = System.in.read()) != -1)) break;
                } catch (IOException e) {
                    e.printStackTrace();
                }
                if (ch != '\n' && ch != '\r')
                {
                    try {
                        int tpack = (Integer.parseInt(String.valueOf((char) ch)) - 1);
                        if (tpack >= 0 && tpack <= (selectedPack.versions.size() - 1)) {
                            selectedVersion = selectedPack.versions.get(tpack);
                            break;
                        } else {
                            System.out.println("Invalid selection, please try again.");
                        }
                    } catch (Exception ignored)
                    {
                        System.out.println("Invalid selection, please try again.");
                    }
                }
            }
            ch = 0;
            System.out.println("This will install '"+selectedPack.name+"' version '"+selectedVersion.name+"' from channel '"+selectedVersion.type+"' to '"+installPath.toAbsolutePath().toString()+"'.");
            System.out.println("If you are unhappy with this installation location, please move this executable to your desired location or use the arguments available by running this with --help to customise your install.");
            System.out.println("Are you sure you wish to continue? [y/n]");
            while (true)
            {
                try {
                    if (!((ch = System.in.read()) != -1)) break;
                } catch (IOException e) {
                    e.printStackTrace();
                }
                if (ch != '\n' && ch != '\r')
                {
                    if(ch != 'y' && ch != 'Y') System.exit(0);
                    break;
                }
            }
            selectedVersion.install();
        } else {
            ServerPack tmp = new ServerPack(expectedPack);
            tmp.downloadManifest();
            ServerVersion selectedVersion = null;
            Main.packs.add(tmp);
            Collections.reverse(tmp.versions);
            for(ServerVersion ver : tmp.versions)
            {
                if(latest) {
                    if (ver.type.equals("Release")) {
                        selectedVersion = ver;
                        break;
                    }
                } else {
                    if(ver.id == expectedVer)
                    {
                        selectedVersion = ver;
                        break;
                    }
                }
            }
            if(selectedVersion == null) {
                System.out.println("Invalid version.");
                System.exit(-4);
            }
            if(autoInstall)
            {
                int ch = 0;
                System.out.println("This will install '"+tmp.name+"' version '"+selectedVersion.name+"' from channel '"+selectedVersion.type+"' to '"+installPath.toAbsolutePath().toString()+"'.");
                System.out.println("Are you sure you wish to continue? [y/n]");
                while (true)
                {
                    try {
                        if (!((ch = System.in.read()) != -1)) break;
                    } catch (IOException e) {
                        e.printStackTrace();
                    }
                    if (ch != '\n' && ch != '\r')
                    {
                        if(ch != 'y' && ch != 'Y') System.exit(0);
                        break;
                    }
                }
            }
            System.out.println("Installing '"+tmp.name+"' version '"+selectedVersion.name+"' from channel '"+selectedVersion.type+"' to '"+installPath.toAbsolutePath().toString()+"'...");
            selectedVersion.install();
        }
	    System.exit(0);
    }

/*    void downloadFiles(File instanceDir, File forgeLibs)
    {
        CreeperLogger.INSTANCE.info("Attempting to downloaded required files");

        ArrayList<CompletableFuture> futures = new ArrayList<>();
        overallBytes.set(0);

        currentBytes.set(0);

        List<DownloadableFile> requiredFiles = null;
        try
        {
            requiredFiles = getRequiredDownloads(new File(instanceDir + File.separator + "version.json"), forgeLibs);
        } catch (MalformedURLException err)
        {
            err.printStackTrace();
            return;
        }
        //Need to loop first for overallBytes or things get weird.
        for (DownloadableFile file : requiredFiles)
        {
            Path path = Paths.get(file.getPath());
            if (!path.toFile().exists())
            {
                if (file.getSize() > 0)
                {
                    overallBytes.addAndGet(file.getSize());
                }
            }
        }
        for (DownloadableFile file : requiredFiles)
        {
            File f = new File(instanceDir + File.separator + file.getPath());
            System.out.println(file.getPath());
            if (!f.exists()) f.mkdir();
            try
            {
                URI url = new URI(file.getUrl());
                Path path = Paths.get(file.getPath());
                if (!path.toFile().exists())
                {
                    DownloadTask task = new DownloadTask(file, path);//url, path, file.getSize(), false, file.getSha1() );
                    futures.add(task.execute());
                }
            } catch (Exception e)
            {
                e.printStackTrace();
            }
        }
        try
        {
            CompletableFuture<Void> combinedFuture = CompletableFuture.allOf(
                    futures.toArray(new CompletableFuture[0])).exceptionally((t) ->
                    {
                        t.printStackTrace();
                        return null;
                    }
            );

            futures.forEach((blah) ->
            {
                ((CompletableFuture<Void>) blah).exceptionally((t) ->
                {
                    combinedFuture.completeExceptionally(t);
                    return null;
                });
            });

            combinedFuture.join();

        } catch (Throwable err) 
        {
            CreeperLogger.INSTANCE.error(err.getMessage());
            for (CompletableFuture ftr : futures)
            {
                ftr.cancel(true);
            }
            throw err;
        }
    }*/

    public static String getDefaultThreadLimit(String arg)
    {
        return String.valueOf((Runtime.getRuntime().availableProcessors() / 2) - 1);
    }

    private List<DownloadableFile> getRequiredDownloads(File file, File forgeLibs) throws MalformedURLException {
        // This is where you give stuffs
        return null;
    }
}
