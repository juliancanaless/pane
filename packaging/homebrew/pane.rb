class Pane < Formula
  desc "Shared local memory and coordination for concurrent coding agents"
  homepage "https://github.com/juliancanaless/pane"
  version "0.1.9"
  license "MIT"
  head "https://github.com/juliancanaless/pane.git", branch: "main"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/juliancanaless/pane/releases/download/v0.1.9/pane-v0.1.9-darwin-arm64.tar.gz"
      sha256 "1fa14ed64315292404bf78e83008fb1c831847e03e7c9c79275ad7b655a2238a"
    else
      url "https://github.com/juliancanaless/pane/releases/download/v0.1.9/pane-v0.1.9-darwin-amd64.tar.gz"
      sha256 "d87ea5c6f4fe1a4ae750d52c36b7d282974e15dcd807a9c2c1164d0f3f10dc94"
    end
  end

  depends_on "go" => :build if build.head?
  depends_on "rust" => :build if build.head?

  def install
    if build.head?
      ENV.prepend_path "PATH", Formula["rust"].opt_bin
      system "make", "build"
      bin.install "bin/pane"
      bin.install "bin/pane-analyze"
    else
      bin.install "pane"
      bin.install "pane-analyze"
    end
  end

  test do
    assert_match "Pane gives concurrent coding agents", shell_output("#{bin}/pane help")
    assert_match "platform:", shell_output("#{bin}/pane doctor")
  end
end
