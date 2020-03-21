package net.creeperhost.creeperlauncher.util;

import net.creeperhost.creeperlauncher.CreeperLogger;

import java.net.*;
import java.time.LocalDateTime;
import java.time.format.DateTimeFormatter;
import java.util.Locale;

public class MiscUtils
{
    public static String byteArrayToHex(byte[] a)
    {
        StringBuilder sb = new StringBuilder(a.length * 2);
        for (byte b : a)
            sb.append(String.format("%02x", b));
        return sb.toString();
    }

    public static URI findForgeDownloadURL(String minecraftVersion, String forgeVersion)
    {
        String repo = "https://dist.creeper.host/versions/net/minecraftforge/forge/";

        URI url = null;
        try {
            url = new URI(repo + minecraftVersion + "-" + forgeVersion + "/" +
                    "forge-" + minecraftVersion + "-" + forgeVersion + "-installer.jar");

            if (!checkExist(url.toURL()))
            {
                url = new URI(repo + minecraftVersion + "-" + forgeVersion + "-" + minecraftVersion + "/" +
                        "forge-" + minecraftVersion + "-" + forgeVersion + "-" + minecraftVersion + "-installer.jar");

                if (!checkExist(url.toURL()))
                {
                    url = new URI(repo + minecraftVersion + "-" + forgeVersion + "/" +
                            "forge-" + minecraftVersion + "-" + forgeVersion + "-installer.zip");
                }

            }
        } catch (URISyntaxException | MalformedURLException e) {
            return null;
        }

        return url;
    }

    public static boolean checkExist(URL url)
    {
        boolean response;
        try
        {
            HttpURLConnection connection = (HttpURLConnection) url.openConnection();
            connection.setRequestMethod("HEAD");
            connection.connect();
            response = ((connection.getResponseCode() == 200) && (connection.getContentLength() >= 0));
            connection.disconnect();
        } catch (Exception err)
        {
            response = false;
        }
        return response;
    }
}
