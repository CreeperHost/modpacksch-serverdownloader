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
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.Executor;
import java.util.concurrent.atomic.AtomicLong;

public class Main {
    public static AtomicLong overallBytes = new AtomicLong(0);
    public static AtomicLong currentBytes = new AtomicLong(0);
    public static Path installPath;
    private static ArrayList<ServerPack> packs = new ArrayList<ServerPack>();
    public static void main(String[] args) {
        boolean search = false;
        installPath = Paths.get("");
        long expectedPack = 0;
        long expectedVer = 0;
        boolean latest = true;
	    try
        {
            //Do we have an pack ID or are we searching for a pack?
            expectedPack = Long.parseLong(args[0]);
        } catch(Exception ignored)
        {
            search = true;
        }
	    if(!search) {
            if (args.length > 1) {
                try {
                    //Do we have a version ID or are we just grabbing the latest?
                    expectedVer = Long.parseLong(args[1]);
                    latest = false;
                } catch (Exception ignored) {
                    latest = true;
                }
            }
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
                    int tpack = (Integer.parseInt(String.valueOf((char)ch))-1);
                    if(tpack >= 0 && tpack <= (packs.size()-1)) {
                        selectedPack = packs.get(tpack);
                        break;
                    } else {
                        System.out.println("Invalid selection, please try again.");
                    }
                }
            }
            System.out.println("Selected '"+selectedPack.name+"'...");
            num = 1;
            System.out.println("Please select a version below:");
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
                    int tpack = (Integer.parseInt(String.valueOf((char)ch))-1);
                    if(tpack >= 0 && tpack <= (selectedPack.versions.size()-1)) {
                        selectedVersion = selectedPack.versions.get(tpack);
                        break;
                    } else {
                        System.out.println("Invalid selection, please try again.");
                    }
                }
            }
            ch = 0;
            System.out.println("This will install '"+selectedPack.name+"' version '"+selectedVersion.name+"' of channel '"+selectedVersion.type+"' to '"+installPath.toAbsolutePath().toString()+"'.");
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
                }
            }
            selectedVersion.install();
        } else {
            ServerPack tmp = new ServerPack(expectedPack);
            tmp.downloadManifest();
            ServerVersion selectedVersion = null;
            Main.packs.add(tmp);
            for(ServerVersion ver : tmp.versions)
            {
                if(latest) {
                    if (ver.type == "Release") {
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
            System.out.println("Installing '"+tmp.name+"' version '"+selectedVersion.name+"' from channel '"+selectedVersion.type+"' to '"+installPath+"'...");
            selectedVersion.install();
        }
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
